# Flag Ownership Design

## Summary

Add optional owner (user) assignment to flags. Owners are displayed on flag list and detail views and serve as the point of contact for flag lifecycle management. Addresses issue #40.

## Design Decisions

- **Single owner per flag** — nullable `owner_id` FK on the `flags` table. YAGNI on multi-owner.
- **ON DELETE SET NULL** — when an owner user is deleted, flags become unowned.
- **Gravatar avatars** — MD5 hash of lowercase email, `?d=mp` default for users without Gravatar.
- **No new endpoints** — ownership is a field on existing flag CRUD (create/update).

## Database

New migration `010_flag_ownership`:

```sql
ALTER TABLE flags ADD COLUMN owner_id UUID REFERENCES users(id) ON DELETE SET NULL;
CREATE INDEX idx_flags_owner_id ON flags(owner_id);
```

## Go Model

```go
type Flag struct {
    // ... existing fields ...
    OwnerID   *string    `json:"owner_id,omitempty"`
    Owner     *FlagOwner `json:"owner,omitempty"`
}

type FlagOwner struct {
    ID          string  `json:"id"`
    Email       string  `json:"email"`
    DisplayName *string `json:"display_name,omitempty"`
}
```

`Owner` is read-only, populated by LEFT JOIN in queries.

## Store Layer

- `ListByProject` and `FindByKey`: LEFT JOIN users to populate `Flag.Owner`.
- New `?owner=` filter parameter on `ListByProject`, matching existing filter pattern (`AND f.owner_id = $N`).
- `Create` and `Update`: accept `owner_id`, include in INSERT/UPDATE SQL.

## Handler Layer

- Create/update flag requests accept optional `owner_id` field.
- List flags accepts `?owner=` query parameter (user UUID).
- Audit log captures owner changes automatically via existing old/new JSON snapshot mechanism.

## Frontend

### Flag List
- Owner filter dropdown populated from users list.
- Owner name/email chip on each flag card alongside existing metadata badges.
- Gravatar avatar (small, inline) next to owner name.

### Flag Detail
- Owner displayed in metadata section with Gravatar avatar.
- Combobox/select to set/change owner (includes "Unassigned" option).
- Display `display_name` if available, fall back to email.

### Gravatar
- Hash: MD5 of `email.trim().toLowerCase()`
- URL: `https://gravatar.com/avatar/{hash}?d=mp&s={size}`

## Testing

- **Store tests**: create/update with owner_id, list with owner filter, owner nullified on user deletion.
- **Handler tests**: owner_id flows through create/update, audit captures changes, filter param passed to store.
