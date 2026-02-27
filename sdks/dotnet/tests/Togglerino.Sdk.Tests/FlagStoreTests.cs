using System.Reactive.Linq;
using System.Text.Json;
using Xunit;

namespace Togglerino.Sdk.Tests;

public class FlagStoreTests
{
    private static EvaluationResult MakeResult(object value, string variant = "v1", string reason = "default")
    {
        var json = JsonSerializer.SerializeToElement(value);
        return new EvaluationResult { Value = json, Variant = variant, Reason = reason };
    }

    [Fact]
    public void Get_ReturnsNull_WhenFlagNotFound()
    {
        var store = new Internal.FlagStore();
        Assert.Null(store.Get("nonexistent"));
    }

    [Fact]
    public void ReplaceAll_PopulatesCache()
    {
        var store = new Internal.FlagStore();
        var flags = new Dictionary<string, EvaluationResult>
        {
            ["flag-a"] = MakeResult(true),
            ["flag-b"] = MakeResult("hello"),
        };

        store.ReplaceAll(flags, emitEvents: false);

        Assert.NotNull(store.Get("flag-a"));
        Assert.NotNull(store.Get("flag-b"));
    }

    [Fact]
    public void ReplaceAll_EmitsChangeEvents_ForChangedFlags()
    {
        var store = new Internal.FlagStore();
        store.ReplaceAll(new Dictionary<string, EvaluationResult>
        {
            ["flag-a"] = MakeResult(true, "on"),
        }, emitEvents: false);

        var changes = new List<FlagChangeEvent>();
        using var sub = store.Changes.Subscribe(changes.Add);

        store.ReplaceAll(new Dictionary<string, EvaluationResult>
        {
            ["flag-a"] = MakeResult(false, "off"),
        }, emitEvents: true);

        Assert.Single(changes);
        Assert.Equal("flag-a", changes[0].FlagKey);
        Assert.Equal("off", changes[0].Variant);
    }

    [Fact]
    public void ReplaceAll_EmitsDeletedEvents_ForRemovedFlags()
    {
        var store = new Internal.FlagStore();
        store.ReplaceAll(new Dictionary<string, EvaluationResult>
        {
            ["flag-a"] = MakeResult(true),
            ["flag-b"] = MakeResult("hello"),
        }, emitEvents: false);

        var deletions = new List<FlagDeletedEvent>();
        using var sub = store.Deletions.Subscribe(deletions.Add);

        store.ReplaceAll(new Dictionary<string, EvaluationResult>
        {
            ["flag-a"] = MakeResult(true),
        }, emitEvents: true);

        Assert.Single(deletions);
        Assert.Equal("flag-b", deletions[0].FlagKey);
    }

    [Fact]
    public void ReplaceAll_NoEvents_WhenValuesUnchanged()
    {
        var store = new Internal.FlagStore();
        var flags = new Dictionary<string, EvaluationResult>
        {
            ["flag-a"] = MakeResult(true, "on"),
        };
        store.ReplaceAll(flags, emitEvents: false);

        var changes = new List<FlagChangeEvent>();
        var deletions = new List<FlagDeletedEvent>();
        using var sub1 = store.Changes.Subscribe(changes.Add);
        using var sub2 = store.Deletions.Subscribe(deletions.Add);

        store.ReplaceAll(new Dictionary<string, EvaluationResult>
        {
            ["flag-a"] = MakeResult(true, "on"),
        }, emitEvents: true);

        Assert.Empty(changes);
        Assert.Empty(deletions);
    }

    [Fact]
    public void ApplyUpdate_UpdatesCacheAndEmitsChange()
    {
        var store = new Internal.FlagStore();
        var changes = new List<FlagChangeEvent>();
        using var sub = store.Changes.Subscribe(changes.Add);

        store.ApplyUpdate("flag-a", MakeResult(42, "answer"));

        Assert.NotNull(store.Get("flag-a"));
        Assert.Single(changes);
        Assert.Equal("flag-a", changes[0].FlagKey);
    }

    [Fact]
    public void ApplyDeletion_RemovesFromCacheAndEmitsDeleted()
    {
        var store = new Internal.FlagStore();
        store.ReplaceAll(new Dictionary<string, EvaluationResult>
        {
            ["flag-a"] = MakeResult(true),
        }, emitEvents: false);

        var deletions = new List<FlagDeletedEvent>();
        using var sub = store.Deletions.Subscribe(deletions.Add);

        store.ApplyDeletion("flag-a");

        Assert.Null(store.Get("flag-a"));
        Assert.Single(deletions);
    }
}
