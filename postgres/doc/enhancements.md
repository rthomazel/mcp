# Possible Enhancements

Ideas that did not make it into the initial release. Prioritised by value-to-effort ratio.

---

## `list_enums`

Enum types are currently invisible. If a column has type `appointment_status`,
the agent has no idea what values are valid without running a raw query.

**Proposed output** (`schema`, `type`, `labels`):

```
schema    type                    labels
public    appointment_status      scheduled, confirmed, completed, cancelled, no_show
public    user_role               admin, clinician, staff, patient
```

**Query sketch:**
```sql
SELECT
    n.nspname AS schema,
    t.typname AS type,
    string_agg(e.enumlabel, ', ' ORDER BY e.enumsortorder) AS labels
FROM pg_catalog.pg_type t
JOIN pg_catalog.pg_enum e ON e.enumtypid = t.oid
JOIN pg_catalog.pg_namespace n ON n.oid = t.typnamespace
WHERE n.nspname = $1
GROUP BY n.nspname, t.typname
ORDER BY t.typname
```

No cap needed — the number of enum types is schema-bounded.

---

## `list_constraints`

Primary keys, unique constraints, and check constraints are not currently
exposed. `list_foreign_keys` covers the FK case but agents still can't answer
"what is the PK of this table?" without running a raw query.

A single `list_constraints` tool could consolidate all constraint types,
making `list_foreign_keys` potentially redundant (keep it for convenience).

**Proposed output** (`constraint`, `type`, `columns`, `expression`):

```
constraint                  type          columns                   expression
appointments_pkey           PRIMARY KEY   id
appointments_status_check   CHECK                                   status IN ('scheduled', ...)
appointments_patient_fk     FOREIGN KEY   patient_id -> patients.id
appointments_slot_unique    UNIQUE        slot_id, provider_id
```

**Query sketch:**
```sql
SELECT
    c.conname AS constraint,
    CASE c.contype
        WHEN 'p' THEN 'PRIMARY KEY'
        WHEN 'u' THEN 'UNIQUE'
        WHEN 'c' THEN 'CHECK'
        WHEN 'f' THEN 'FOREIGN KEY'
    END AS type,
    string_agg(a.attname, ', ' ORDER BY x.n) AS columns,
    pg_get_constraintdef(c.oid, true) AS expression
FROM pg_catalog.pg_constraint c
JOIN pg_catalog.pg_class r ON r.oid = c.conrelid
JOIN pg_catalog.pg_namespace n ON n.oid = r.relnamespace
LEFT JOIN LATERAL unnest(c.conkey) WITH ORDINALITY AS x(attnum, n) ON true
LEFT JOIN pg_catalog.pg_attribute a
    ON a.attrelid = c.conrelid AND a.attnum = x.attnum
WHERE n.nspname = $1
  AND ($2::text IS NULL OR r.relname = $2)
GROUP BY c.conname, c.contype, c.oid
ORDER BY r.relname, c.contype, c.conname
```

The `table` parameter is optional — omit for all constraints in the schema, pass a
table name to filter to one table.

No cap needed — constraint counts are schema-bounded.
