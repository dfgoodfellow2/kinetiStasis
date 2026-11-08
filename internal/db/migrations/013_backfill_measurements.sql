-- +goose Up
-- Legacy body_measurements rows predate the nullable-circumference contract:
-- the circumference columns are REAL with no default, so rows inserted before a
-- column existed (shoulders/calves arrived in 003) carry SQL NULL. Readers now
-- COALESCE defensively, but normalising the stored data keeps every consumer
-- (exports, ad-hoc SQL, future joins) on the same 0 = "not logged" footing.
-- +goose StatementBegin
UPDATE body_measurements SET neck_cm      = 0 WHERE neck_cm      IS NULL;
UPDATE body_measurements SET chest_cm     = 0 WHERE chest_cm     IS NULL;
UPDATE body_measurements SET waist_cm     = 0 WHERE waist_cm     IS NULL;
UPDATE body_measurements SET hips_cm      = 0 WHERE hips_cm      IS NULL;
UPDATE body_measurements SET thigh_cm     = 0 WHERE thigh_cm     IS NULL;
UPDATE body_measurements SET bicep_cm     = 0 WHERE bicep_cm     IS NULL;
UPDATE body_measurements SET shoulders_cm = 0 WHERE shoulders_cm IS NULL;
UPDATE body_measurements SET calves_cm    = 0 WHERE calves_cm    IS NULL;
UPDATE body_measurements SET notes        = '' WHERE notes       IS NULL;
-- +goose StatementEnd

-- +goose Down
-- Deliberately a no-op. The backfill only rewrites NULL to the documented
-- "not logged" sentinel (0 for circumferences, '' for notes), and that mapping
-- is lossless in the direction that matters: a NULL and a 0 both mean "no value
-- was recorded". Restoring NULLs would not recover any information and would
-- only reintroduce the scan hazard this migration exists to remove.
