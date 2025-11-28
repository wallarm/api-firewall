// file: query_engine_all_paths_schema32.test.js
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

// В спеке servers: [] — задаём вручную или через env
const BASE_URL = __ENV.BASE_URL || 'http://localhost:8282';
const DATASET_ID = __ENV.DATASET_ID || 'test-dataset';

// Общие заголовки
const H = { 'X-Wallarm-Schema-ID': '32' };
const H_JSON = { ...H, 'Content-Type': 'application/json' };

// Ожидаемый стаб-ответ
const EXPECTED = { summary: [{ schema_id: 32, status_code: 200 }] };

// Helpers
function j(r) { try { return r.json(); } catch { return null; } }
function eq(a, b) { return JSON.stringify(a) === JSON.stringify(b); }
function u(p) { return `${BASE_URL}${p}`; }

const op = new Trend('qe_op_ms');

export default function () {
    group('Query Engine API – all endpoints return stub (schema_id=32)', () => {
        // 1) GET /api/v1/query-engine/datasets
        {
            const res = http.get(u(`/api/v1/query-engine/datasets`), { headers: H });
            op.add(res.timings.duration, { path: '/datasets', method: 'GET' });
            const body = j(res);
            check(res, { 'GET /datasets -> 200': (r) => r.status === 200 });
            check(body, { 'GET /datasets -> expected body': (b) => eq(b, EXPECTED) });
            console.log('Response body:', body);
        }

        // 2) GET /api/v1/query-engine/datasets/{id}
        {
            const res = http.get(u(`/api/v1/query-engine/datasets/${encodeURIComponent(DATASET_ID)}`), { headers: H });
            op.add(res.timings.duration, { path: '/datasets/{id}', method: 'GET' });
            const body = j(res);
            check(res, { 'GET /datasets/{id} -> 200': (r) => r.status === 200 });
            check(body, { 'GET /datasets/{id} -> expected body': (b) => eq(b, EXPECTED) });
            console.log('Response body:', body);
        }

        // 3) POST /api/v1/query-engine/datasets/{id}/query  (без тела)
        {
            const res = http.post(u(`/api/v1/query-engine/datasets/${encodeURIComponent(DATASET_ID)}/query`), '', { headers: H_JSON });
            op.add(res.timings.duration, { path: '/datasets/{id}/query', method: 'POST' });
            const body = j(res);
            check(res, { 'POST /datasets/{id}/query -> 200': (r) => r.status === 200 });
            check(body, { 'POST /datasets/{id}/query -> expected body': (b) => eq(b, EXPECTED) });
            console.log('Response body:', body);
        }

        // 4) POST /api/v1/query-engine/datasets/{id}/query с query-параметрами
        {
            const params = { headers: H_JSON };
            const url = u(`/api/v1/query-engine/datasets/${encodeURIComponent(DATASET_ID)}/query?limit=1&cursor=abc&direction=NEXT`);
            const res = http.post(url, '', params);
            op.add(res.timings.duration, { path: '/datasets/{id}/query?params', method: 'POST' });
            const body = j(res);
            check(res, { 'POST /datasets/{id}/query?params -> 200': (r) => r.status === 200 });
            check(body, { 'POST /datasets/{id}/query?params -> expected body': (b) => eq(b, EXPECTED) });
            console.log('Response body:', body);
        }

        // 5) POST /api/v1/query-engine/datasets/{id}/export  (без тела)
        {
            const res = http.post(u(`/api/v1/query-engine/datasets/${encodeURIComponent(DATASET_ID)}/export`), '', { headers: H_JSON });
            op.add(res.timings.duration, { path: '/datasets/{id}/export', method: 'POST' });
            const body = j(res);
            check(res, { 'POST /datasets/{id}/export -> 200': (r) => r.status === 200 });
            check(body, { 'POST /datasets/{id}/export -> expected body': (b) => eq(b, EXPECTED) });
            console.log('Response body:', body);
        }

        // 6) POST /api/v1/query-engine/datasets/{id}/aggregate  (без тела)
        {
            const res = http.post(u(`/api/v1/query-engine/datasets/${encodeURIComponent(DATASET_ID)}/aggregate`), '', { headers: H_JSON });
            op.add(res.timings.duration, { path: '/datasets/{id}/aggregate', method: 'POST' });
            const body = j(res);
            check(res, { 'POST /datasets/{id}/aggregate -> 200': (r) => r.status === 200 });
            check(body, { 'POST /datasets/{id}/aggregate -> expected body': (b) => eq(b, EXPECTED) });
            console.log('Response body:', body);
        }

        // 7) POST /api/v1/query-engine/datasets/{id}/query/count  (без тела)
        {
            const res = http.post(u(`/api/v1/query-engine/datasets/${encodeURIComponent(DATASET_ID)}/query/count`), '', { headers: H_JSON });
            op.add(res.timings.duration, { path: '/datasets/{id}/query/count', method: 'POST' });
            const body = j(res);
            check(res, { 'POST /datasets/{id}/query/count -> 200': (r) => r.status === 200 });
            check(body, { 'POST /datasets/{id}/query/count -> expected body': (b) => eq(b, EXPECTED) });
            console.log('Response body:', body);
        }
    });
}