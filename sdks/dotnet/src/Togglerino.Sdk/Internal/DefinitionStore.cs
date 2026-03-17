namespace Togglerino.Sdk.Internal;

/// <summary>
/// Thread-safe store for flag and segment definitions used by server-side evaluation.
/// Uses ReaderWriterLockSlim to allow concurrent reads with exclusive writes.
/// </summary>
internal sealed class DefinitionStore : IDisposable
{
    private readonly ReaderWriterLockSlim _lock = new();
    private List<FlagDefinition> _flags = new();
    private List<SegmentDefinition> _segments = new();

    /// <summary>
    /// Replaces all flag and segment definitions under a write lock.
    /// </summary>
    public void Update(List<FlagDefinition> flags, List<SegmentDefinition> segments)
    {
        _lock.EnterWriteLock();
        try
        {
            _flags = flags;
            _segments = segments;
        }
        finally
        {
            _lock.ExitWriteLock();
        }
    }

    /// <summary>
    /// Returns a copy of the current flag definitions under a read lock.
    /// </summary>
    public List<FlagDefinition> GetFlags()
    {
        _lock.EnterReadLock();
        try
        {
            return new List<FlagDefinition>(_flags);
        }
        finally
        {
            _lock.ExitReadLock();
        }
    }

    /// <summary>
    /// Returns a copy of the current segment definitions under a read lock.
    /// </summary>
    public List<SegmentDefinition> GetSegments()
    {
        _lock.EnterReadLock();
        try
        {
            return new List<SegmentDefinition>(_segments);
        }
        finally
        {
            _lock.ExitReadLock();
        }
    }

    public void Dispose()
    {
        _lock.Dispose();
    }
}
