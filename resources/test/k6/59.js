// file: kotlin_service_template_schema34_allof_oneof.test.js
import http from 'k6/http';
import { check, group } from 'k6';

export const options = {
    vus: Number(__ENV.VUS || 1),
    iterations: Number(__ENV.ITERATIONS || 1),
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8282';
const H = {
    'X-Wallarm-Schema-ID': '59',
    'rl-tenant-id': __ENV.RL_TENANT_ID || 'tenant-1',
    'rl-user-id': __ENV.RL_USER_ID || 'user-1',
};
const H_JSON = { ...H, 'Content-Type': 'application/json' };

// Стаб, который возвращают мок/фильтр Wallarm. Если видим его — пропускаем проверки формы виджета
const EXPECTED_STUB = { summary: [{ schema_id: 59, status_code: 200 }] };

function j(r) { try { return r.json(); } catch { return null; } }
function eq(a, b) { return JSON.stringify(a) === JSON.stringify(b); }
function u(p) { return `${BASE_URL}${p}`; }
function uuid() {
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, c=>{
        const r=(Math.random()*16)|0, v=c==='x'?r:(r&0x3|0x8); return v.toString(16);
    });
}

// -------- Helpers to validate allOf(widgetId + widgetProperties) --------
function isString(x) { return typeof x === 'string' && x.length >= 0; }
function checkAllOfWidget(obj) {
    // widget = allOf(widgetId, widgetProperties) + required: id, name, type, description
    return obj
        && isString(obj.id)
        && isString(obj.name)
        && isString(obj.type)
        && isString(obj.description);
}

// -------- Bodies --------
const createBody = (overrides={}) => ({
    name: 'Widget A',
    type: 'BASIC',
    description: 'Example',
    ...overrides,
});
const updateBody = (overrides={}) => ({
    description: 'Updated',
    ...overrides,
});

// =================== POSITIVE: проверка allOf ===================
export default function () {
    group('ALLOF – Create -> Get -> Update -> Delete', () => {
        // POST /widgets  (Create)
        const rCreate = http.post(u('/api-internal/v1/widgets'), JSON.stringify(createBody()), { headers: H_JSON });
        check(rCreate, {
            'POST /widgets 201/200': r => r.status === 201 || r.status === 200,
        });
        console.log('Response body:', rCreate.body);

        const bjCreate = j(rCreate);
        if (bjCreate && !eq(bjCreate, EXPECTED_STUB)) {
            // Если не стаб — проверяем allOf
            check(bjCreate, { 'create matches allOf widget': b => checkAllOfWidget(b) });
        }

        // Вытащим id если сервер реально вернул виджет, иначе сгенерим
        const id = (bjCreate && bjCreate.id && !eq(bjCreate, EXPECTED_STUB)) ? bjCreate.id : uuid();

        // GET /widgets/{id}
        const rGet = http.get(u(`/api-internal/v1/widgets/${encodeURIComponent(id)}`), { headers: H });
        check(rGet, { 'GET /widgets/{id} 200/404': r => [200,404].includes(r.status) });
        const bjGet = j(rGet);
        if (rGet.status === 200 && bjGet && !eq(bjGet, EXPECTED_STUB)) {
            check(bjGet, { 'get matches allOf widget': b => checkAllOfWidget(b) });
        }
        console.log('Response body:', rGet.body);

        // PATCH /widgets/{id}
        const rPatch = http.patch(u(`/api-internal/v1/widgets/${encodeURIComponent(id)}`), JSON.stringify(updateBody()), { headers: H_JSON });
        check(rPatch, { 'PATCH /widgets/{id} 200/404': r => [200,404].includes(r.status) });
        const bjPatch = j(rPatch);
        if (rPatch.status === 200 && bjPatch && !eq(bjPatch, EXPECTED_STUB)) {
            check(bjPatch, { 'patch matches allOf widget': b => checkAllOfWidget(b) });
        }
        console.log('Response body:', rPatch.body);

        // DELETE /widgets/{id}
        const rDel = http.del(u(`/api-internal/v1/widgets/${encodeURIComponent(id)}`), null, { headers: H });
        check(rDel, { 'DELETE /widgets/{id} 204/200/404': r => [204,200,404].includes(r.status) });
        console.log('Response body:', rDel.body);
    });

    // =================== NEGATIVE: нарушаем allOf (required из обеих частей) ===================
    group('ALLOF – Negative create (missing required fields)', () => {
        // Нет type
        const r1 = http.post(u('/api-internal/v1/widgets'), JSON.stringify(createBody({ type: undefined })), { headers: H_JSON });
        check(r1, { 'missing type -> 400/non-2xx': r => r.status === 400 || r.status < 200 || r.status >= 300 });
        console.log('Response body:', r1.body);

        // Нет name
        const r2 = http.post(u('/api-internal/v1/widgets'), JSON.stringify(createBody({ name: undefined })), { headers: H_JSON });
        check(r2, { 'missing name -> 400/non-2xx': r => r.status === 400 || r.status < 200 || r.status >= 300 });
        console.log('Response body:', r2.body);

        // Нет description
        const r3 = http.post(u('/api-internal/v1/widgets'), JSON.stringify(createBody({ description: undefined })), { headers: H_JSON });
        check(r3, { 'missing description -> 400/non-2xx': r => r.status === 400 || r.status < 200 || r.status >= 300 });
        console.log('Response body:', r3.body);
    });

    // =================== ONEOF: примеры (в текущей спеки НЕТ oneOf) ===================
    // Ниже — шаблон, как тестировать oneOf, если добавишь схему с oneOf в components.
    /*
    // Пример схемы (для справки, не исполняется):
    // components:
    //   schemas:
    //     widgetUpsert:
    //       oneOf:
    //         - $ref: '#/components/schemas/widgetCreate'  # required: name,type,description
    //         - $ref: '#/components/schemas/widgetUpdate'  # required: id + (любые свойства)
    //
    // Тесты:
    group('ONEOF – Positive branch #1 (create variant)', () => {
      const body = { name: 'W', type: 'BASIC', description: 'D' }; // соответствует widgetCreate
      const r = http.post(u('/api-internal/v1/widgets/upsert'), JSON.stringify(body), { headers: H_JSON });
      check(r, { 'oneOf create branch -> 200/201': rr => [200,201].includes(rr.status) });
    });

    group('ONEOF – Positive branch #2 (update variant)', () => {
      const body = { id: uuid(), description: 'upd only' }; // соответствует widgetUpdate
      const r = http.post(u('/api-internal/v1/widgets/upsert'), JSON.stringify(body), { headers: H_JSON });
      check(r, { 'oneOf update branch -> 200/201': rr => [200,201].includes(rr.status) });
    });

    group('ONEOF – Negative (matches none or multiple)', () => {
      // 1) Не подходит ни под одну ветку: нет обязательных полей обеих веток
      const rBad1 = http.post(u('/api-internal/v1/widgets/upsert'), JSON.stringify({ foo: 'bar' }), { headers: H_JSON });
      check(rBad1, { 'oneOf none -> 400/non-2xx': rr => rr.status === 400 || rr.status < 200 || rr.status >= 300 });

      // 2) Подходит под обе ветки (если такое возможно по схеме) — сервер должен отбраковать
      const rBad2 = http.post(u('/api-internal/v1/widgets/upsert'), JSON.stringify({ id: uuid(), name: 'W', type: 'BASIC', description: 'D' }), { headers: H_JSON });
      check(rBad2, { 'oneOf multiple -> 400/non-2xx': rr => rr.status === 400 || rr.status < 200 || rr.status >= 300 });
    });
    */
}