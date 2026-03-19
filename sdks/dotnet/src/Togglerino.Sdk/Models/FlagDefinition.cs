using System.Text.Json;
using System.Text.Json.Serialization;

namespace Togglerino.Sdk;

/// <summary>
/// Represents a flag definition from the definitions API.
/// </summary>
public record FlagDefinition(
    [property: JsonPropertyName("key")] string Key,
    [property: JsonPropertyName("valueType")] string ValueType,
    [property: JsonPropertyName("status")] string Status,
    [property: JsonPropertyName("defaultValue")] JsonElement? DefaultValue,
    [property: JsonPropertyName("variants")] List<VariantDefinition> Variants,
    [property: JsonPropertyName("config")] FlagDefinitionConfig Config
);

/// <summary>
/// Per-environment configuration for a flag definition.
/// </summary>
public record FlagDefinitionConfig(
    [property: JsonPropertyName("enabled")] bool Enabled,
    [property: JsonPropertyName("fallthroughVariant")] string FallthroughVariant,
    [property: JsonPropertyName("offVariant")] string? OffVariant,
    [property: JsonPropertyName("targetingRules")] List<TargetingRuleDefinition> TargetingRules
);

/// <summary>
/// A variant with its value as a raw JSON element.
/// </summary>
public record VariantDefinition(
    [property: JsonPropertyName("name")] string Name,
    [property: JsonPropertyName("value")] JsonElement Value
);

/// <summary>
/// A targeting rule with conditions, variant, and optional percentage rollout.
/// </summary>
public record TargetingRuleDefinition(
    [property: JsonPropertyName("variant")] string Variant,
    [property: JsonPropertyName("percentage")] int? Percentage,
    [property: JsonPropertyName("conditions")] List<ConditionDefinition> Conditions
);

/// <summary>
/// A condition within a targeting rule or segment.
/// </summary>
public record ConditionDefinition(
    [property: JsonPropertyName("attribute")] string Attribute,
    [property: JsonPropertyName("operator")] string Operator,
    [property: JsonPropertyName("value")] string Value
);

/// <summary>
/// A reusable segment with conditions.
/// </summary>
public record SegmentDefinition(
    [property: JsonPropertyName("key")] string Key,
    [property: JsonPropertyName("conditions")] List<ConditionDefinition> Conditions
);

/// <summary>
/// Response from the definitions API.
/// </summary>
public record DefinitionsResponse(
    [property: JsonPropertyName("flags")] List<FlagDefinition> Flags,
    [property: JsonPropertyName("segments")] List<SegmentDefinition> Segments
);
