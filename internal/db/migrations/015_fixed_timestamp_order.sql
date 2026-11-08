-- +goose Up
-- Normalise resource timestamps to one fixed 9-digit fractional layout so that
-- the lexical ORDER BY created_at used by target history (and the same column
-- in check_in_logs) is chronological. Historical rows were written as RFC3339
-- seconds ("...:05Z"); newer rows used RFC3339Nano, whose variable-length
-- fraction can compare out of order inside the same second. The writers now
-- emit constants.FormatResourceTimestamp and this migration rewrites every
-- existing Z-suffixed value into that exact shape.
--
-- Values with a non-Z offset, empty strings or other unparseable shapes are
-- left untouched: no writer produces them, and guessing a zone would be worse
-- than preserving the original bytes.
-- +goose StatementBegin
UPDATE target_history
SET created_at = CASE
    WHEN instr(created_at, '.') = 0
        THEN substr(created_at, 1, 19) || '.000000000Z'
    WHEN created_at LIKE '%.%Z'
        THEN substr(created_at, 1, 19) || '.' ||
             substr(substr(created_at, 21, instr(created_at, 'Z') - 21) || '000000000', 1, 9) || 'Z'
    ELSE created_at
END
WHERE created_at LIKE '____-__-__T__:__:__%Z';

UPDATE check_in_logs
SET created_at = CASE
    WHEN instr(created_at, '.') = 0
        THEN substr(created_at, 1, 19) || '.000000000Z'
    WHEN created_at LIKE '%.%Z'
        THEN substr(created_at, 1, 19) || '.' ||
             substr(substr(created_at, 21, instr(created_at, 'Z') - 21) || '000000000', 1, 9) || 'Z'
    ELSE created_at
END
WHERE created_at LIKE '____-__-__T__:__:__%Z';
-- +goose StatementEnd

-- +goose Down
-- Deliberately a no-op. The normalisation is idempotent and preserves the
-- instant; re-expanding fixed fractions back to variable/omitted ones would
-- reintroduce the ordering ambiguity (and could not faithfully reconstruct the
-- original byte shapes).
