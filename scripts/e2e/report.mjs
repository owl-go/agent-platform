import { readFileSync, writeFileSync } from 'node:fs';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { cases } from './cases.mjs';
const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const output = resolve(root, 'output/playwright');
const report = JSON.parse(readFileSync(resolve(output, 'results.json'), 'utf8'));
function specs(suites) { return suites.flatMap(s => [...s.specs, ...specs(s.suites ?? [])]); }
const actual = new Map(specs(report.suites).map(s => [s.title.split(' | ')[0], s]));
if (actual.size !== cases.length || cases.some(c => !actual.has(c.id))) throw new Error('Case catalog and Playwright report do not match');
const results = cases.map(c => {
  const s = actual.get(c.id), result = s.tests[0].results.at(-1);
  const status = result?.status ?? 'notRun';
  return { ...c, status, durationMs: result?.duration ?? 0, line: s.line };
});
const summary = { startTime: report.stats.startTime, durationMs: report.stats.duration, total: results.length, passed: results.filter(r => r.status === 'passed').length, failed: results.filter(r => r.status === 'failed').length, skipped: results.filter(r => r.status === 'skipped').length, results };
writeFileSync(resolve(output, 'summary.json'), JSON.stringify(summary, null, 2) + '\n');
console.log(JSON.stringify({ total: summary.total, passed: summary.passed, failed: summary.failed, skipped: summary.skipped }));
