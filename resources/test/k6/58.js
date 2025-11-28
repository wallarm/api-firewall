// file: oas31_feature_showcase.test.js
import http from 'k6/http';
import { check, group } from 'k6';
import { Trend } from 'k6/metrics';

export const options = {
    vus: Number(__ENV.VUS || 1),
    iterations: Number(__ENV.ITERATIONS || 1),
    thresholds: {
        http_req_failed: ['rate==0'],
        http_req_duration: ['p(95)<1000'],
    },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8282';

// общие заголовки и куки
const H = {
    'X-Wallarm-Schema-ID': '58',
    'Cookie': 'SESSIONID=dummy',
};
const H_JSON = { ...H, 'Content-Type': 'application/json' };

// ожидаемый стаб
const EXPECTED = { summary: [{ schema_id: 58, status_code: 200 }] };

function j(r) { try { return r.json(); } catch { return null; } }
function eq(a, b) { return JSON.stringify(a) === JSON.stringify(b); }
function u(p) { return `${BASE_URL}${p}`; }

const op = new Trend('oas31_op_ms');

// ------------------ Позитивные кейсы ------------------
function positives() {
    group('POSITIVE: /v1/orders (JSON Schema 2020-12 features)', () => {
        const body = {
            id: 'ord-1',
            status: 'NEW',            // const
            note: 'deliver asap',     // becomes required when flags.express = true (if/then)
            items: [
                { sku: 'SKU-1', qty: 1 } // unevaluatedProperties: false enforced in item
            ],
            flags: { express: true }  // triggers if/then -> note required
        };
        const res = http.post(u('/v1/orders'), JSON.stringify(body), { headers: H_JSON });
        op.add(res.timings.duration, { path: '/v1/orders', method: 'POST', kind: 'positive' });
        const bodyJson = j(res);
        check(res, { '200 /v1/orders': (r) => r.status === 200 });
        check(bodyJson, { 'expected stub /v1/orders': (b) => eq(b, EXPECTED) });
        console.log('Response body:', res.body);
    });

    group('POSITIVE: /v1/orders/{id} (deepObject + allowReserved)', () => {
        const params = '?filter[date][gt]=2025-01-01&filter[date][lt]=2025-12-31&search=[abc]{def}';
        const res = http.get(u(`/v1/orders/ord-1${params}`), { headers: H });
        op.add(res.timings.duration, { path: '/v1/orders/{id}', method: 'GET', kind: 'positive' });
        const bodyJson = j(res);
        check(res, { '200 /v1/orders/{id}': (r) => r.status === 200 });
        check(bodyJson, { 'expected stub /v1/orders/{id}': (b) => eq(b, EXPECTED) });
        console.log('Response body:', res.body);
    });

    group('POSITIVE: /v1/profile (JSON and merge-patch)', () => {
        const resJson = http.patch(u('/v1/profile'), JSON.stringify({ nickname: 'Nick', age: 33 }), { headers: H_JSON });
        op.add(resJson.timings.duration, { path: '/v1/profile', method: 'PATCH', kind: 'positive-json' });
        check(resJson, { '200 /v1/profile JSON': (r) => r.status === 200 });
        check(j(resJson), { 'expected stub /v1/profile JSON': (b) => eq(b, EXPECTED) });
        console.log('Response body:', resJson.body);

        const resPatch = http.patch(u('/v1/profile'), JSON.stringify({ nickname: null }), { headers: { ...H, 'Content-Type': 'application/merge-patch+json' } });
        op.add(resPatch.timings.duration, { path: '/v1/profile', method: 'PATCH', kind: 'positive-merge' });
        check(resPatch, { '200 /v1/profile merge-patch': (r) => r.status === 200 });
        check(j(resPatch), { 'expected stub /v1/profile merge-patch': (b) => eq(b, EXPECTED) });
        console.log('Response body:', resPatch.body);
    });

    group('POSITIVE: /v1/refsibling ($ref with siblings)', () => {
        // $ref -> codeBase + minLength sibling
        const res = http.post(u('/v1/refsibling'), JSON.stringify({ code: 'ABC_1' }), { headers: H_JSON });
        op.add(res.timings.duration, { path: '/v1/refsibling', method: 'POST', kind: 'positive' });
        check(res, { '200 /v1/refsibling': (r) => r.status === 200 });
        check(j(res), { 'expected stub /v1/refsibling': (b) => eq(b, EXPECTED) });
        console.log('Response body:', res.body);
    });
}

// ------------------ Негативные кейсы ------------------
function negatives() {
    group('NEGATIVE: /v1/orders -> const violation (status != NEW)', () => {
        const body = {
            id: 'ord-2',
            status: 'OLD',         // должно быть NEW
            items: [{ sku: 'SKU-1', qty: 1 }],
            flags: { express: false }
        };
        const res = http.post(u('/v1/orders'), JSON.stringify(body), { headers: H_JSON });
        op.add(res.timings.duration, { path: '/v1/orders', method: 'POST', kind: 'neg-const' });
        const bj = j(res);
        check(res, { 'non-200 on const violation': (r) => r.status !== 200 });
        check(bj,  { 'body != stub on const violation': (b) => !eq(b, EXPECTED) });
    });

    group('NEGATIVE: /v1/orders -> if/then/else (express=true but note missing)', () => {
        const body = {
            id: 'ord-3',
            status: 'NEW',
            items: [{ sku: 'SKU-1', qty: 1 }],
            flags: { express: true } // note отсутствует
        };
        const res = http.post(u('/v1/orders'), JSON.stringify(body), { headers: H_JSON });
        op.add(res.timings.duration, { path: '/v1/orders', method: 'POST', kind: 'neg-if-then' });
        check(res, { 'non-200 on if/then violation': (r) => r.status !== 200 });
        check(j(res), { 'body != stub on if/then violation': (b) => !eq(b, EXPECTED) });
    });

    group('NEGATIVE: /v1/orders -> unevaluatedProperties (extra field in item)', () => {
        const body = {
            id: 'ord-4',
            status: 'NEW',
            items: [{ sku: 'SKU-1', qty: 1, extra: 'nope' }], // лишнее поле
        };
        const res = http.post(u('/v1/orders'), JSON.stringify(body), { headers: H_JSON });
        op.add(res.timings.duration, { path: '/v1/orders', method: 'POST', kind: 'neg-unevaluated' });
        check(res, { 'non-200 on unevaluatedProperties': (r) => r.status !== 200 });
        check(j(res), { 'body != stub on unevaluatedProperties': (b) => !eq(b, EXPECTED) });
    });

    group('NEGATIVE: /v1/profile -> wrong type (age < 0)', () => {
        const res = http.patch(u('/v1/profile'), JSON.stringify({ age: -1 }), { headers: H_JSON });
        op.add(res.timings.duration, { path: '/v1/profile', method: 'PATCH', kind: 'neg-age' });
        check(res, { 'non-200 on min violation': (r) => r.status !== 200 });
        check(j(res), { 'body != stub on min violation': (b) => !eq(b, EXPECTED) });
    });

    group('NEGATIVE: /v1/refsibling -> $ref sibling minLength not satisfied', () => {
        const res = http.post(u('/v1/refsibling'), JSON.stringify({ code: 'A1' }), { headers: H_JSON }); // слишком короткий
        op.add(res.timings.duration, { path: '/v1/refsibling', method: 'POST', kind: 'neg-ref-sibling' });
        check(res, { 'non-200 on ref+minLength violation': (r) => r.status !== 200 });
        check(j(res), { 'body != stub on ref+minLength violation': (b) => !eq(b, EXPECTED) });

    });

    group('NEGATIVE: missing Cookie SESSIONID (security)', () => {
        const res = http.get(u('/v1/orders/ord-1?search=[abc]'), { headers: { 'X-Wallarm-Schema-ID': '999' } });
        op.add(res.timings.duration, { path: '/v1/orders/{id}', method: 'GET', kind: 'neg-missing-cookie' });
        check(res, { 'non-200 without cookie': (r) => r.status !== 200 });
        check(j(res), { 'body != stub without cookie': (b) => !eq(b, EXPECTED) });
    });
}

export default function () {
    positives();
    negatives();
}