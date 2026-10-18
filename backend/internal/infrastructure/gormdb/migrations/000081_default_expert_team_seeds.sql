ALTER TABLE default_resource_seeds DROP CONSTRAINT default_resource_seeds_kind_check;
ALTER TABLE default_resource_seeds ADD CONSTRAINT default_resource_seeds_kind_check
CHECK(kind IN ('skill','expert','expert_team','connector'));
