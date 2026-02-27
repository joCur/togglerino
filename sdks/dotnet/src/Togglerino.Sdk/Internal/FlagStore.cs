using System.Collections.Concurrent;
using System.Reactive.Subjects;
using System.Text.Json;

namespace Togglerino.Sdk.Internal;

internal sealed class FlagStore : IDisposable
{
    private readonly ConcurrentDictionary<string, EvaluationResult> _flags = new();
    private readonly Subject<FlagChangeEvent> _changes = new();
    private readonly Subject<FlagDeletedEvent> _deletions = new();

    public IObservable<FlagChangeEvent> Changes => _changes;
    public IObservable<FlagDeletedEvent> Deletions => _deletions;

    public EvaluationResult? Get(string key)
    {
        return _flags.GetValueOrDefault(key);
    }

    public void ReplaceAll(Dictionary<string, EvaluationResult> newFlags, bool emitEvents)
    {
        if (emitEvents)
        {
            // Detect deleted flags
            foreach (var key in _flags.Keys)
            {
                if (!newFlags.ContainsKey(key))
                {
                    _flags.TryRemove(key, out _);
                    _deletions.OnNext(new FlagDeletedEvent(key));
                }
            }

            // Detect new or changed flags
            foreach (var (key, newResult) in newFlags)
            {
                if (_flags.TryGetValue(key, out var oldResult))
                {
                    if (!ValuesEqual(oldResult, newResult))
                    {
                        _flags[key] = newResult;
                        _changes.OnNext(new FlagChangeEvent(key, newResult.Value, newResult.Variant));
                    }
                }
                else
                {
                    _flags[key] = newResult;
                    _changes.OnNext(new FlagChangeEvent(key, newResult.Value, newResult.Variant));
                }
            }
        }
        else
        {
            _flags.Clear();
            foreach (var (key, result) in newFlags)
            {
                _flags[key] = result;
            }
        }
    }

    public void ApplyUpdate(string flagKey, EvaluationResult result)
    {
        _flags[flagKey] = result;
        _changes.OnNext(new FlagChangeEvent(flagKey, result.Value, result.Variant));
    }

    public void ApplyDeletion(string flagKey)
    {
        if (_flags.TryRemove(flagKey, out _))
        {
            _deletions.OnNext(new FlagDeletedEvent(flagKey));
        }
    }

    public void Dispose()
    {
        _changes.OnCompleted();
        _changes.Dispose();
        _deletions.OnCompleted();
        _deletions.Dispose();
    }

    private static bool ValuesEqual(EvaluationResult a, EvaluationResult b)
    {
        return a.Variant == b.Variant
            && a.Reason == b.Reason
            && a.Value.GetRawText() == b.Value.GetRawText();
    }
}
