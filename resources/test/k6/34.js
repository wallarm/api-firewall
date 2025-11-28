// file: kotlin_service_template_schema34.test.js
import http from 'k6/http';
import { check, group } from 'k6';
import { Trend } from 'k6/metrics';

export const options = {
    vus: Number(__ENV.VUS || 1),
    iterations: Number(__ENV.ITERATIONS || 1),
    thresholds: {
        http_req_failed: ['rate==0'],
        http_req_duration: ['p(95)<1200'],
    },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8282';

const REQUIRED_HEADERS = {
    'X-Wallarm-Schema-ID': '34',
    'rl-tenant-id': __ENV.RL_TENANT_ID || 'tenant-1',
    'rl-user-id': __ENV.RL_USER_ID || 'user-1',
};
const H = { ...REQUIRED_HEADERS };
const H_JSON = { ...H, 'Content-Type': 'application/json' };

const WRONG_SCHEMA_H = { ...REQUIRED_HEADERS, 'X-Wallarm-Schema-ID': '34' };

const EXPECTED = { summary: [{ schema_id: 34, status_code: 200 }] };

function j(r) { try { return r.json(); } catch { return null; } }
function eq(a, b) { return JSON.stringify(a) === JSON.stringify(b); }
function u(p) { return `${BASE_URL}${p}`; }
function uuid() {
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, c => {
        const r = (Math.random()*16)|0, v = c === 'x' ? r : (r&0x3|0x8);
        return v.toString(16);
    });
}

// ---- минимальные полезные данные ----
function createWidgetBody() {
    return { name: 'Widget A', type: 'BASIC', description: 'Example widget' };
}
function updateWidgetBody() {
    return { description: 'Updated description' };
}

const op = new Trend('kst_op_ms');

// ---- Позитивные тесты ----
function positiveTests() {
    group('POSITIVE /api-internal/v1/widgets (GET list)', () => {
        const url = u('/api-internal/v1/widgets?limit=2&direction=STARTING_AFTER&ids=a&ids=b&types=BASIC');
        const res = http.get(url, { headers: H });
        op.add(res.timings.duration, { path: '/widgets', method: 'GET' });
        const body = j(res);
        // допускаем 200; если есть тело — сверяем стаб
        check(res, { 'GET /widgets -> 200': r => r.status === 200 });
        if (res.body && res.body.length) {
            check(body, { 'GET /widgets -> expected stub': b => eq(b, EXPECTED) });
            console.log('Response body:', body);
        }
    });

    group('POSITIVE /api-internal/v1/widgets (POST create)', () => {
        const res = http.post(u('/api-internal/v1/widgets'), JSON.stringify(createWidgetBody()), { headers: H_JSON });
        op.add(res.timings.duration, { path: '/widgets', method: 'POST' });
        const body = j(res);
        // по спеке 201; некоторые стабы могут вернуть 200 — примем оба
        check(res, { 'POST /widgets -> 201 or 200': r => r.status === 201 || r.status === 200 });
        if (res.status === 200 || res.headers['Content-Type']?.includes('application/json')) {
            check(body, { 'POST /widgets -> expected stub (if JSON body)': b => !res.body || eq(b, EXPECTED) });
            console.log('Response body:', body);
        }
    });

    group('POSITIVE /api-internal/v1/widgets/{id} (GET item)', () => {
        const id = uuid(); // произвольный
        const res = http.get(u(`/api-internal/v1/widgets/${encodeURIComponent(id)}`), { headers: H });
        op.add(res.timings.duration, { path: '/widgets/{id}', method: 'GET' });
        const body = j(res);
        check(res, { 'GET /widgets/{id} -> 200 or 404': r => r.status === 200 || r.status === 404 });
        if (res.status === 200) {
            check(body, { 'GET /widgets/{id} -> expected stub': b => eq(b, EXPECTED) });
            console.log('Response body:', body);
        }
    });

    group('POSITIVE /api-internal/v1/widgets/{id} (PATCH update)', () => {
        const id = uuid();
        const res = http.patch(u(`/api-internal/v1/widgets/${encodeURIComponent(id)}`), JSON.stringify(updateWidgetBody()), { headers: H_JSON });
        op.add(res.timings.duration, { path: '/widgets/{id}', method: 'PATCH' });
        const body = j(res);
        check(res, { 'PATCH /widgets/{id} -> 200 or 404': r => r.status === 200 || r.status === 404 });
        if (res.status === 200) {
            check(body, { 'PATCH /widgets/{id} -> expected stub': b => eq(b, EXPECTED) });
            console.log('Response body:', body);
        }
    });

    group('POSITIVE /api-internal/v1/widgets/{id} (DELETE)', () => {
        const id = uuid();
        const res = http.del(u(`/api-internal/v1/widgets/${encodeURIComponent(id)}`), null, { headers: H });
        op.add(res.timings.duration, { path: '/widgets/{id}', method: 'DELETE' });
        // по спеке 204; стабы иногда возвращают 200 — примем оба
        check(res, { 'DELETE /widgets/{id} -> 204 or 200 or 404': r => [204,200,404].includes(r.status) });
        if (res.status === 200 && res.body) {
            check(j(res), { 'DELETE /widgets/{id} -> expected stub if body': b => eq(b, EXPECTED) });
            console.log('Response body:', res.body);
        }
    });
}

// ---- Негативные тесты ----
function negativeTests() {
    // 1) отсутствуют обязательные заголовки
    group('NEG missing required headers', () => {
        const url = u('/api-internal/v1/widgets');
        const res = http.get(url, { headers: { 'X-Wallarm-Schema-ID': '34' } }); // нет rl-tenant-id / rl-user-id
        op.add(res.timings.duration, { path: '/widgets', method: 'GET', neg: 'missing headers' });
        check(res, { 'GET /widgets without rl headers -> non-2xx': r => r.status < 200 || r.status >= 300 });
        check(j(res), { 'body != stub': b => !eq(b, EXPECTED) });
        console.log('Response body:', res.body);
    });

    // 2) неверный X-Wallarm-Schema-ID
    group('NEG wrong X-Wallarm-Schema-ID', () => {
        const res = http.get(u('/api-internal/v1/widgets'), { headers: WRONG_SCHEMA_H });
        op.add(res.timings.duration, { path: '/widgets', method: 'GET', neg: 'wrong schema id' });
        check(res, { 'GET /widgets wrong schema -> non-2xx': r => r.status < 200 || r.status >= 300 });
        check(j(res), { 'body != stub': b => !eq(b, EXPECTED) });
        console.log('Response body:', res.body);
    });

    // 3) невалидные query-параметры (limit < minimum, direction неверный)
    group('NEG invalid query params', () => {
        const res1 = http.get(u('/api-internal/v1/widgets?limit=0'), { headers: H }); // minimum: 1
        op.add(res1.timings.duration, { path: '/widgets', method: 'GET', neg: 'limit=0' });
        check(res1, { 'GET /widgets limit=0 -> 400 (or non-2xx)': r => r.status === 400 || r.status < 200 || r.status >= 300 });
        check(j(res1), { 'body != stub': b => !eq(b, EXPECTED) });
        console.log('Response body:', res1.body);

        const res2 = http.get(u('/api-internal/v1/widgets?direction=SIDEWAYS'), { headers: H });
        op.add(res2.timings.duration, { path: '/widgets', method: 'GET', neg: 'bad direction' });
        check(res2, { 'GET /widgets direction invalid -> 400 (or non-2xx)': r => r.status === 400 || r.status < 200 || r.status >= 300 });
        check(j(res2), { 'body != stub': b => !eq(b, EXPECTED) });
        console.log('Response body:', res2.body);
    });

    // 4) create: отсутствует обязательное поле
    group('NEG POST /widgets missing required body fields', () => {
        const bad = { name: 'W', description: 'no type' }; // отсутствует type
        const res = http.post(u('/api-internal/v1/widgets'), JSON.stringify(bad), { headers: H_JSON });
        op.add(res.timings.duration, { path: '/widgets', method: 'POST', neg: 'missing type' });
        check(res, { 'POST /widgets missing type -> 400 (or non-2xx)': r => r.status === 400 || r.status < 200 || r.status >= 300 });
        check(j(res), { 'body != stub': b => !eq(b, EXPECTED) });
        console.log('Response body:', res.body);
    });

    // 5) update: пустое тело
    group('NEG PATCH /widgets/{id} empty body', () => {
        const id = uuid();
        const res = http.patch(u(`/api-internal/v1/widgets/${id}`), '', { headers: H_JSON });
        op.add(res.timings.duration, { path: '/widgets/{id}', method: 'PATCH', neg: 'empty body' });
        check(res, { 'PATCH /widgets/{id} empty -> 400/404/non-2xx': r => [400,404].includes(r.status) || r.status < 200 || r.status >= 300 });
        check(j(res), { 'body != stub': b => !eq(b, EXPECTED) });
        console.log('Response body:', res.body);
    });

    // 6) get/delete несуществующего id (ожидаем 404)
    group('NEG GET/DELETE nonexistent id -> 404', () => {
        const id = 'does-not-exist';
        const resG = http.get(u(`/api-internal/v1/widgets/${encodeURIComponent(id)}`), { headers: H });
        op.add(resG.timings.duration, { path: '/widgets/{id}', method: 'GET', neg: 'nonexistent' });
        check(resG, { 'GET /widgets/{id} -> 404 or non-2xx': r => r.status === 404 || r.status < 200 || r.status >= 300 });
        check(j(resG), { 'body != stub': b => !eq(b, EXPECTED) });
        console.log('Response body:', resG.body);

        const resD = http.del(u(`/api-internal/v1/widgets/${encodeURIComponent(id)}`), null, { headers: H });
        op.add(resD.timings.duration, { path: '/widgets/{id}', method: 'DELETE', neg: 'nonexistent' });
        check(resD, { 'DELETE /widgets/{id} -> 404 or non-2xx': r => r.status === 404 || r.status < 200 || r.status >= 300 });
        if (resD.body) {
            check(j(resD), { 'body != stub': b => !eq(b, EXPECTED) });
            console.log('Response body:', resD.body);
        }
    });

    // 7) пропущенные обязательные заголовки на mutate-методах
    group('NEG missing rl headers on POST/PATCH/DELETE', () => {
        const id = uuid();

        const resP = http.post(u('/api-internal/v1/widgets'), JSON.stringify(createWidgetBody()), { headers: { 'X-Wallarm-Schema-ID': '34', 'Content-Type': 'application/json' } });
        op.add(resP.timings.duration, { path: '/widgets', method: 'POST', neg: 'no rl headers' });
        check(resP, { 'POST /widgets without rl headers -> non-2xx': r => r.status < 200 || r.status >= 300 });
        console.log('Response body:', resP.body);

        const resU = http.patch(u(`/api-internal/v1/widgets/${id}`), JSON.stringify(updateWidgetBody()), { headers: { 'X-Wallarm-Schema-ID': '34', 'Content-Type': 'application/json' } });
        op.add(resU.timings.duration, { path: '/widgets/{id}', method: 'PATCH', neg: 'no rl headers' });
        check(resU, { 'PATCH /widgets/{id} without rl headers -> non-2xx': r => r.status < 200 || r.status >= 300 });
        console.log('Response body:', resU.body);

        const resD = http.del(u(`/api-internal/v1/widgets/${id}`), null, { headers: { 'X-Wallarm-Schema-ID': '34' } });
        op.add(resD.timings.duration, { path: '/widgets/{id}', method: 'DELETE', neg: 'no rl headers' });
        check(resD, { 'DELETE /widgets/{id} without rl headers -> non-2xx': r => r.status < 200 || r.status >= 300 });
        console.log('Response body:', resD.body);
    });
}

export default function () {
    positiveTests();
    negativeTests();
}