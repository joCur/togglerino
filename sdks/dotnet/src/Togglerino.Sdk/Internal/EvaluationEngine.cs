using System.Buffers.Binary;
using System.Globalization;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using System.Text.RegularExpressions;

namespace Togglerino.Sdk.Internal;

/// <summary>
/// Local flag evaluation engine for server-side SDKs.
/// Ports the Go backend's evaluation logic (internal/evaluation/engine.go).
/// </summary>
internal static class EvaluationEngine
{
    /// <summary>
    /// Evaluates a flag for a given context.
    /// </summary>
    /// <param name="flag">The flag definition from the definitions API.</param>
    /// <param name="context">The evaluation context (userId + attributes).</param>
    /// <param name="segments">List of segment definitions for segment_match resolution.</param>
    /// <returns>The evaluation result with value, variant, and reason.</returns>
    public static EvaluationResult Evaluate(
        FlagDefinition flag,
        EvaluationContext? context,
        List<SegmentDefinition>? segments)
    {
        var ctx = context ?? new EvaluationContext();
        var segMap = BuildSegmentMap(segments);

        // Boolean flags use a simplified evaluation path.
        if (flag.ValueType == "boolean")
        {
            return EvaluateBoolean(flag, ctx, segMap);
        }

        var config = flag.Config;

        // 1. If flag is archived, return default value with reason "archived".
        if (flag.Status == "archived")
        {
            return new EvaluationResult
            {
                Value = LookupVariantValue(config.Variants, config.DefaultVariant),
                Variant = "",
                Reason = "archived",
            };
        }

        // 2. If config is disabled, return default value with reason "disabled".
        if (!config.Enabled)
        {
            return new EvaluationResult
            {
                Value = LookupVariantValue(config.Variants, config.DefaultVariant),
                Variant = "",
                Reason = "disabled",
            };
        }

        // 3. Evaluate targeting rules in order (first match wins).
        foreach (var rule in config.TargetingRules)
        {
            if (MatchesAllConditions(rule.Conditions, ctx, segMap))
            {
                // Check percentage rollout.
                if (rule.Percentage is not null)
                {
                    var bucket = ConsistentHash(flag.Key, ctx.UserId ?? "");
                    if (bucket >= rule.Percentage.Value)
                    {
                        // User is outside the rollout percentage; continue to next rule.
                        continue;
                    }
                }

                // Rule matched — find the variant value.
                var value = LookupVariantValue(config.Variants, rule.Variant);
                return new EvaluationResult
                {
                    Value = value,
                    Variant = rule.Variant,
                    Reason = "rule_match",
                };
            }
        }

        // 4. Return default variant.
        return new EvaluationResult
        {
            Value = LookupVariantValue(config.Variants, config.DefaultVariant),
            Variant = config.DefaultVariant,
            Reason = "default",
        };
    }

    /// <summary>
    /// Simplified evaluation path for boolean flags.
    /// For boolean flags: enabled = true, disabled = false, archived = false.
    /// Targeting rules use "true"/"false" strings as variant keys.
    /// </summary>
    internal static EvaluationResult EvaluateBoolean(
        FlagDefinition flag,
        EvaluationContext ctx,
        Dictionary<string, SegmentDefinition> segments)
    {
        if (flag.Status == "archived")
        {
            return new EvaluationResult
            {
                Value = ToJsonElement(false),
                Variant = "",
                Reason = "archived",
            };
        }

        if (!flag.Config.Enabled)
        {
            return new EvaluationResult
            {
                Value = ToJsonElement(false),
                Variant = "",
                Reason = "disabled",
            };
        }

        // Evaluate targeting rules.
        foreach (var rule in flag.Config.TargetingRules)
        {
            if (MatchesAllConditions(rule.Conditions, ctx, segments))
            {
                if (rule.Percentage is not null)
                {
                    var bucket = ConsistentHash(flag.Key, ctx.UserId ?? "");
                    if (bucket >= rule.Percentage.Value)
                    {
                        continue;
                    }
                }

                return new EvaluationResult
                {
                    Value = ToJsonElement(rule.Variant == "true"),
                    Variant = "",
                    Reason = "rule_match",
                };
            }
        }

        // Default: enabled = true.
        return new EvaluationResult
        {
            Value = ToJsonElement(true),
            Variant = "",
            Reason = "default",
        };
    }

    /// <summary>
    /// Computes a consistent hash bucket (0-99) for percentage rollouts.
    /// Uses SHA-256 of flagKey+userId, first 8 bytes as big-endian uint64, mod 100.
    /// </summary>
    internal static int ConsistentHash(string flagKey, string userId)
    {
        var input = Encoding.UTF8.GetBytes(flagKey + userId);
        var hash = SHA256.HashData(input);
        var n = BinaryPrimitives.ReadUInt64BigEndian(hash.AsSpan(0, 8));
        return (int)(n % 100);
    }

    /// <summary>
    /// Checks if all conditions in a rule match the evaluation context.
    /// segment_match conditions look up the segment by key and evaluate its conditions.
    /// Passing null for segments in the recursive call prevents nesting.
    /// </summary>
    internal static bool MatchesAllConditions(
        List<ConditionDefinition> conditions,
        EvaluationContext ctx,
        Dictionary<string, SegmentDefinition>? segments)
    {
        foreach (var cond in conditions)
        {
            if (cond.Operator == "segment_match")
            {
                var segKey = cond.Value;
                if (string.IsNullOrEmpty(segKey)) return false;
                if (segments is null) return false;
                if (!segments.TryGetValue(segKey, out var seg)) return false;
                // Evaluate segment conditions (pass null for segments to prevent nesting).
                if (!MatchesAllConditions(seg.Conditions, ctx, null)) return false;
                continue;
            }

            var attrValue = GetContextValue(ctx, cond.Attribute);
            if (!EvaluateCondition(cond, attrValue)) return false;
        }

        return true;
    }

    /// <summary>
    /// Gets a context value for the given attribute.
    /// Maps "user_id" to context.UserId; other attributes come from context.Attributes.
    /// </summary>
    private static object? GetContextValue(EvaluationContext ctx, string attribute)
    {
        if (attribute == "user_id")
        {
            return ctx.UserId;
        }

        if (ctx.Attributes is not null && ctx.Attributes.TryGetValue(attribute, out var value))
        {
            return value;
        }

        return null;
    }

    /// <summary>
    /// Evaluates a single condition against an attribute value.
    /// Implements all 16 operators matching the Go backend.
    /// </summary>
    internal static bool EvaluateCondition(ConditionDefinition condition, object? attributeValue)
    {
        var conditionValue = condition.Value;
        var op = condition.Operator;

        return op switch
        {
            "equals" => ToString(attributeValue) == conditionValue,
            "not_equals" => ToString(attributeValue) != conditionValue,
            "contains" => EvalContains(attributeValue, conditionValue),
            "not_contains" => !EvalContains(attributeValue, conditionValue),
            "starts_with" => ToString(attributeValue).StartsWith(conditionValue, StringComparison.Ordinal),
            "ends_with" => ToString(attributeValue).EndsWith(conditionValue, StringComparison.Ordinal),
            "greater_than" => ToDoublePair(attributeValue, conditionValue, out var a1, out var b1) && a1 > b1,
            "less_than" => ToDoublePair(attributeValue, conditionValue, out var a2, out var b2) && a2 < b2,
            "gte" => ToDoublePair(attributeValue, conditionValue, out var a3, out var b3) && a3 >= b3,
            "lte" => ToDoublePair(attributeValue, conditionValue, out var a4, out var b4) && a4 <= b4,
            "in" => EvalIn(attributeValue, conditionValue),
            "not_in" => !EvalIn(attributeValue, conditionValue),
            "exists" => attributeValue is not null,
            "not_exists" => attributeValue is null,
            "matches" => EvalMatches(attributeValue, conditionValue),
            _ => false,
        };
    }

    /// <summary>
    /// Converts a value to its string representation (matching Go's fmt.Sprintf("%v", v)).
    /// </summary>
    private static string ToString(object? value)
    {
        if (value is null) return "";

        // Handle JsonElement
        if (value is JsonElement je)
        {
            return je.ValueKind switch
            {
                JsonValueKind.String => je.GetString() ?? "",
                JsonValueKind.Number => je.GetRawText(),
                JsonValueKind.True => "True",
                JsonValueKind.False => "False",
                JsonValueKind.Null => "",
                _ => je.GetRawText(),
            };
        }

        return value.ToString() ?? "";
    }

    /// <summary>
    /// Attempts to convert a value to double.
    /// </summary>
    private static bool ToDouble(object? value, out double result)
    {
        result = 0;
        if (value is null) return false;

        if (value is JsonElement je)
        {
            if (je.ValueKind == JsonValueKind.Number)
            {
                result = je.GetDouble();
                return true;
            }
            if (je.ValueKind == JsonValueKind.String)
            {
                return double.TryParse(je.GetString(), CultureInfo.InvariantCulture, out result);
            }
            return false;
        }

        if (value is double d) { result = d; return true; }
        if (value is float f) { result = f; return true; }
        if (value is int i) { result = i; return true; }
        if (value is long l) { result = l; return true; }
        if (value is decimal dec) { result = (double)dec; return true; }

        if (value is string s)
        {
            return double.TryParse(s, CultureInfo.InvariantCulture, out result);
        }

        return false;
    }

    /// <summary>
    /// Converts both values to double for comparison.
    /// </summary>
    private static bool ToDoublePair(object? a, object? b, out double fa, out double fb)
    {
        var okA = ToDouble(a, out fa);
        var okB = ToDouble(b, out fb);
        return okA && okB;
    }

    /// <summary>
    /// Checks if the attribute contains the condition value.
    /// For strings, checks substring containment.
    /// </summary>
    private static bool EvalContains(object? attributeValue, string conditionValue)
    {
        return ToString(attributeValue).Contains(conditionValue, StringComparison.Ordinal);
    }

    /// <summary>
    /// Checks if the attribute value is in the condition list.
    /// The condition value is a JSON-encoded array string (e.g. "[\"US\",\"CA\"]").
    /// </summary>
    private static bool EvalIn(object? attributeValue, string conditionValue)
    {
        try
        {
            var list = JsonSerializer.Deserialize<List<JsonElement>>(conditionValue);
            if (list is null) return false;

            var target = ToString(attributeValue);
            foreach (var item in list)
            {
                var itemStr = item.ValueKind == JsonValueKind.String
                    ? item.GetString() ?? ""
                    : item.GetRawText();
                if (itemStr == target) return true;
            }

            return false;
        }
        catch
        {
            return false;
        }
    }

    /// <summary>
    /// Checks if the attribute value matches a regex pattern.
    /// </summary>
    private static bool EvalMatches(object? attributeValue, string pattern)
    {
        try
        {
            return Regex.IsMatch(ToString(attributeValue), pattern);
        }
        catch
        {
            return false;
        }
    }

    /// <summary>
    /// Finds the value for a variant key in the variants list.
    /// Returns a default JsonElement if the variant is not found.
    /// </summary>
    private static JsonElement LookupVariantValue(List<VariantDefinition> variants, string variantKey)
    {
        foreach (var v in variants)
        {
            if (v.Key == variantKey) return v.Value;
        }

        return default;
    }

    /// <summary>
    /// Builds a Dictionary from a list of segments for efficient lookup.
    /// </summary>
    private static Dictionary<string, SegmentDefinition> BuildSegmentMap(List<SegmentDefinition>? segments)
    {
        var map = new Dictionary<string, SegmentDefinition>();
        if (segments is not null)
        {
            foreach (var s in segments)
            {
                map[s.Key] = s;
            }
        }

        return map;
    }

    /// <summary>
    /// Converts a boolean value to a JsonElement.
    /// </summary>
    internal static JsonElement ToJsonElement(bool value)
    {
        using var doc = JsonDocument.Parse(value ? "true" : "false");
        return doc.RootElement.Clone();
    }

    /// <summary>
    /// Converts an arbitrary value to a JsonElement via serialization.
    /// </summary>
    internal static JsonElement ToJsonElement(object? value)
    {
        var json = JsonSerializer.Serialize(value);
        using var doc = JsonDocument.Parse(json);
        return doc.RootElement.Clone();
    }
}
