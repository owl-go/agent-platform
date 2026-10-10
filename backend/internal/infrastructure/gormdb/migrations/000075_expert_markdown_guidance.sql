ALTER TABLE experts ADD COLUMN guidance text NOT NULL DEFAULT '';

-- Preserve authored text verbatim. Introduction is display-only.
UPDATE experts SET guidance = concat_ws(E'\n\n',
    CASE WHEN core_capability <> '' THEN E'# Core Capability\n\n' || core_capability END,
    CASE WHEN operating_procedure <> '' THEN E'# Operating Procedure\n\n' || operating_procedure END,
    CASE WHEN output_standard <> '' THEN E'# Output Standard\n\n' || output_standard END,
    CASE WHEN cautions <> '' THEN E'# Cautions\n\n' || cautions END,
    CASE WHEN execution_instruction <> '' AND execution_instruction <> operating_procedure
         THEN E'# Execution Instruction\n\n' || execution_instruction END
);
