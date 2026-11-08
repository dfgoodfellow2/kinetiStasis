-- +goose Up
-- Legacy rows in the three log tables predate the non-null numeric contract:
-- biometric_logs' metrics, nutrition_logs.meal_notes and check_in_logs' 004
-- columns were all declared nullable. Readers now COALESCE every one of them,
-- but normalising the stored data keeps exports, ad-hoc SQL and future joins on
-- the same 0 / '' = "not logged" footing.
-- +goose StatementBegin
UPDATE biometric_logs SET weight_kg       = 0  WHERE weight_kg       IS NULL;
UPDATE biometric_logs SET waist_cm        = 0  WHERE waist_cm        IS NULL;
UPDATE biometric_logs SET grip_kg         = 0  WHERE grip_kg         IS NULL;
UPDATE biometric_logs SET bolt_score      = 0  WHERE bolt_score      IS NULL;
UPDATE biometric_logs SET sleep_hours     = 0  WHERE sleep_hours     IS NULL;
UPDATE biometric_logs SET sleep_quality   = 0  WHERE sleep_quality   IS NULL;
UPDATE biometric_logs SET subjective_feel = 0  WHERE subjective_feel IS NULL;
UPDATE biometric_logs SET body_fat_pct    = 0  WHERE body_fat_pct    IS NULL;
UPDATE biometric_logs SET notes           = '' WHERE notes           IS NULL;

UPDATE nutrition_logs SET meal_notes = '' WHERE meal_notes IS NULL;

UPDATE check_in_logs SET weight_before   = 0  WHERE weight_before   IS NULL;
UPDATE check_in_logs SET weight_after    = 0  WHERE weight_after    IS NULL;
UPDATE check_in_logs SET calories_before = 0  WHERE calories_before IS NULL;
UPDATE check_in_logs SET calories_after  = 0  WHERE calories_after  IS NULL;
UPDATE check_in_logs SET reason          = '' WHERE reason          IS NULL;
-- +goose StatementEnd

-- +goose Down
-- Deliberately a no-op. The backfill maps NULL to the documented "not logged"
-- sentinel (0 for numerics, '' for text), which is lossless in the direction
-- that matters: both values mean "no value was recorded". Restoring NULLs
-- would recover no information and would only reintroduce the scan hazard this
-- migration exists to remove.
