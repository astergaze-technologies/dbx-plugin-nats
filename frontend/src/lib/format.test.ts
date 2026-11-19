import assert from "node:assert/strict";
import { test } from "node:test";
import { num, payloadText } from "./format.ts";

const show = (data: string, truncated = false, size = data.length) => payloadText({ data, encoding: "utf8", size, truncated });
const compact = (s: string) => s.replace(/\n\s*/g, "").replace(/": /g, '":');

test("issue payload keeps int64 snowflake IDs exact", () => {
  const raw =
    '{"taskId":864786760355909,"organizationId":"323425076823199749","assetId":323439282914070533,"mimeType":"text/html","urlId":"f5232ca7211595f0af8149492764033310dd5d8a723b76807eb588ff2e1ab1d7","contentId":816983156080709}';
  const shown = show(raw);
  assert.match(shown, /"assetId": 323439282914070533,/);
  assert.equal(compact(shown), raw);
});

test("numbers are shown exactly as stored", () => {
  for (const raw of [
    '{"id":9007199254740993}',
    '{"id":123456789012345678901234567890}',
    '{"id":-9223372036854775808}',
    '{"id":18446744073709551615}',
    '{"x":0.1000000000000000055511151231257827}',
    '{"x":1.0,"y":1e3,"z":2E-7,"w":-0}',
    '[323439282914070533,[323439282914070534,{"n":1e400}]]',
    "323439282914070533",
  ]) assert.equal(compact(show(raw)), raw, raw);
});

test("pretty layout matches JSON.stringify for ordinary JSON", () => {
  for (const v of [{ a: 1, b: [1, { c: null }], d: { e: "x,y:{}" } }, { empty: {}, list: [] }, [1, "two", [3]], "s", null]) {
    assert.equal(show(JSON.stringify(v)), JSON.stringify(v, null, 2));
  }
});

test("randomized: numbers survive formatting", () => {
  let seed = 7;
  const rand = (n: number) => (seed = (seed * 1103515245 + 12345) % 2 ** 31) % n;
  const nums = ["323439282914070533", "-0", "1.0", "1e400", "0.30000000000000004", "18446744073709551615", "-1.50E+10"];
  const gen = (d: number): string => {
    if (d > 3 || rand(3) === 0) return nums[rand(nums.length)];
    const items = Array.from({ length: rand(4) }, (_, i) => gen(d + 1));
    return rand(2) ? `[${items.join(",")}]` : `{${items.map((v, i) => `"k${i}":${v}`).join(",")}}`;
  };
  for (let i = 0; i < 500; i++) {
    const raw = gen(0);
    assert.equal(compact(show(raw)), raw);
  }
});

test("non-JSON, duplicate-key, truncated and binary payloads are shown raw", () => {
  assert.equal(show("hello, {not json"), "hello, {not json");
  assert.equal(show('{"a":1,"a":2}'), '{"a":1,"a":2}');
  assert.equal(show('{"id":323439282914070533,"cut', true, 100000), '{"id":323439282914070533,"cut\n… truncated (97.7 KB total)');
  assert.equal(payloadText({ data: "AAEC", encoding: "base64", size: 3 }), "[binary, 3 B] base64:\nAAEC");
});

test("num formats string sequences without rounding", () => {
  assert.equal(num("1152921504606846977"), BigInt("1152921504606846977").toLocaleString());
  assert.equal(num(1234), (1234).toLocaleString());
  assert.equal(num(undefined), "0");
});
