ALTER TABLE experts
    ADD COLUMN icon text NOT NULL DEFAULT 'sparkles',
    ADD COLUMN icon_background text NOT NULL DEFAULT 'sage',
    ADD COLUMN introduction text NOT NULL DEFAULT '',
    ADD COLUMN core_capability text NOT NULL DEFAULT '',
    ADD COLUMN operating_procedure text NOT NULL DEFAULT '',
    ADD COLUMN output_standard text NOT NULL DEFAULT '',
    ADD COLUMN cautions text NOT NULL DEFAULT '';

UPDATE experts
SET introduction = capability_introduction,
    operating_procedure = execution_instruction;
