using System.Text.Json;
using System.Text.Json.Serialization;
using Togglerino.Sdk.Internal;
using Xunit;

namespace Togglerino.Sdk.Tests;

public class EvaluationEngineTests
{
    /// <summary>
    /// Test case deserialized from testdata/evaluation_cases.json.
    /// </summary>
    private sealed record TestCase
    {
        [JsonPropertyName("name")]
        public string Name { get; init; } = "";

        [JsonPropertyName("flag")]
        public FlagDefinition Flag { get; init; } = null!;

        [JsonPropertyName("segments")]
        public List<SegmentDefinition> Segments { get; init; } = new();

        [JsonPropertyName("context")]
        public TestContext Context { get; init; } = new();

        [JsonPropertyName("expected")]
        public TestExpected Expected { get; init; } = new();
    }

    private sealed record TestContext
    {
        [JsonPropertyName("userId")]
        public string UserId { get; init; } = "";

        [JsonPropertyName("attributes")]
        public Dictionary<string, JsonElement> Attributes { get; init; } = new();
    }

    private sealed record TestExpected
    {
        [JsonPropertyName("value")]
        public JsonElement Value { get; init; }

        [JsonPropertyName("variant")]
        public string Variant { get; init; } = "";

        [JsonPropertyName("reason")]
        public string Reason { get; init; } = "";
    }

    public static IEnumerable<object[]> GetTestCases()
    {
        var json = File.ReadAllText("testdata/evaluation_cases.json");
        var cases = JsonSerializer.Deserialize<List<TestCase>>(json)
            ?? throw new InvalidOperationException("Failed to deserialize test cases");

        foreach (var tc in cases)
        {
            yield return new object[] { tc.Name, tc };
        }
    }

    [Theory]
    [MemberData(nameof(GetTestCases))]
    public void EvaluationEngine_SharedFixtures(string name, object tcObj)
    {
        var tc = (TestCase)tcObj;

        // Convert TestContext to EvaluationContext.
        // The backend reads ctx.Attributes["user_id"] for targeting rules,
        // so we set both UserId and the user_id attribute.
        var attributes = new Dictionary<string, object?>();
        foreach (var (key, value) in tc.Context.Attributes)
        {
            attributes[key] = ConvertJsonElement(value);
        }

        // Always map userId to user_id attribute so targeting rules on "user_id" work.
        if (!string.IsNullOrEmpty(tc.Context.UserId))
        {
            attributes["user_id"] = tc.Context.UserId;
        }

        var context = new EvaluationContext
        {
            UserId = tc.Context.UserId,
            Attributes = attributes,
        };

        var result = EvaluationEngine.Evaluate(tc.Flag, context, tc.Segments);

        Assert.Equal(tc.Expected.Reason, result.Reason);
        Assert.Equal(tc.Expected.Variant, result.Variant);
        AssertJsonElementsEqual(tc.Expected.Value, result.Value, name);
    }

    [Fact]
    public void ConsistentHash_IsDeterministic()
    {
        var hash1 = EvaluationEngine.ConsistentHash("flag-key", "user-1");
        var hash2 = EvaluationEngine.ConsistentHash("flag-key", "user-1");
        Assert.Equal(hash1, hash2);
    }

    [Fact]
    public void ConsistentHash_ReturnsValueInRange()
    {
        for (var i = 0; i < 100; i++)
        {
            var hash = EvaluationEngine.ConsistentHash("flag", $"user-{i}");
            Assert.InRange(hash, 0, 99);
        }
    }

    /// <summary>
    /// Converts a JsonElement to a plain CLR object for use as an attribute value.
    /// </summary>
    private static object? ConvertJsonElement(JsonElement element)
    {
        return element.ValueKind switch
        {
            JsonValueKind.String => element.GetString(),
            JsonValueKind.Number => element.GetDouble(),
            JsonValueKind.True => true,
            JsonValueKind.False => false,
            JsonValueKind.Null => null,
            _ => element.GetRawText(),
        };
    }

    /// <summary>
    /// Compares two JsonElement values for semantic equality.
    /// </summary>
    private static void AssertJsonElementsEqual(JsonElement expected, JsonElement actual, string testName)
    {
        if (expected.ValueKind != actual.ValueKind)
        {
            // Allow number comparison where kinds might differ slightly.
            if (expected.ValueKind == JsonValueKind.Number && actual.ValueKind == JsonValueKind.Number)
            {
                Assert.Equal(expected.GetDouble(), actual.GetDouble());
                return;
            }

            Assert.Fail(
                $"[{testName}] Value kind mismatch: expected {expected.ValueKind} ({expected.GetRawText()}), " +
                $"got {actual.ValueKind} ({actual.GetRawText()})");
        }

        switch (expected.ValueKind)
        {
            case JsonValueKind.True:
            case JsonValueKind.False:
                Assert.Equal(expected.GetBoolean(), actual.GetBoolean());
                break;
            case JsonValueKind.String:
                Assert.Equal(expected.GetString(), actual.GetString());
                break;
            case JsonValueKind.Number:
                Assert.Equal(expected.GetDouble(), actual.GetDouble());
                break;
            case JsonValueKind.Object:
                Assert.Equal(
                    JsonSerializer.Serialize(expected),
                    JsonSerializer.Serialize(actual));
                break;
            case JsonValueKind.Array:
                Assert.Equal(
                    JsonSerializer.Serialize(expected),
                    JsonSerializer.Serialize(actual));
                break;
            default:
                Assert.Equal(expected.GetRawText(), actual.GetRawText());
                break;
        }
    }
}
