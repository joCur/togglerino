using System.Text.Json;

namespace Togglerino.Sdk;

public record FlagChangeEvent(string FlagKey, JsonElement Value, string Variant);
