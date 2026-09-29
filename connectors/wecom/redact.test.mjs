import assert from 'node:assert/strict';
import test from 'node:test';
import { redact } from './redact.mjs';

test('redacts every exact credential value from CLI output', () => {
  const output = 'token=T-123; secret=S-456; bot=B-789; again=T-123';
  assert.equal(redact(output, ['S-456', 'T-123', 'B-789']), 'token=[REDACTED]; secret=[REDACTED]; bot=[REDACTED]; again=[REDACTED]');
});

test('handles absent output and empty values', () => {
  assert.equal(redact(null, ['', undefined]), '');
});
