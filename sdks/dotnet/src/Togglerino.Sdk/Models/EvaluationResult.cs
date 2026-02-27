using System.Text.Json;
using System.Text.Json.Serialization;

namespace Togglerino.Sdk;

public record EvaluationResult
{
    [JsonPropertyName("value")]
    public JsonElement Value { get; init; }

    [JsonPropertyName("variant")]
    public string Variant { get; init; } = "";

    [JsonPropertyName("reason")]
    public string Reason { get; init; } = "";
}
