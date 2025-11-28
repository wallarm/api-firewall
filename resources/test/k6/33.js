// file: recon_v3_all_paths_schema33_with_negative.test.js
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

// --- Headers
const H = { 'X-Wallarm-Schema-ID': '33' };
const H_JSON = { ...H, 'Content-Type': 'application/json' };
const H_WRONG = { 'X-Wallarm-Schema-ID': '33' };
const H_JSON_WRONG = { ...H_WRONG, 'Content-Type': 'application/json' };

// --- Expected stub
const EXPECTED = { summary: [{ schema_id: 33, status_code: 200 }] };

// --- Helpers
function j(r) { try { return r.json(); } catch { return null; } }
function eq(a, b) { return JSON.stringify(a) === JSON.stringify(b); }
function u(p) { return `${BASE_URL}${p}`; }
function uuidv4() {
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, c => {
        const r = (Math.random() * 16) | 0, v = c === 'x' ? r : (r & 0x3 | 0x8);
        return v.toString(16);
    });
}
function doRequest(method, url, headers, body) {
    if (method === 'GET') return http.get(url, { headers });
    if (method === 'DELETE') return http.del(url, null, { headers });
    if (method === 'POST') return http.post(url, body ? JSON.stringify(body) : '', { headers });
    if (method === 'PATCH') return http.patch(url, body ? JSON.stringify(body) : '', { headers });
    throw new Error(`Unsupported method ${method}`);
}

const opTrend = new Trend('recon_op_ms');

// --- Minimal valid payloads (по required полям схем)
function minimalCustodianSetting() {
    return {
        customName: 'rs-min',
        reconcileDate: 'TODAY',
        updatePriceAtStartOfDay: false,
        excludeInternalAssetTypes: [],
        excludeInternalInstrumentTypes: [],
        excludeInternalTransactionTypes: [],
        positionReconcileDateType: 'TRADE_DATE',
        positionReconcileFaceType: 'CURRENT_QUANTITY',
        positionReconcileValueType: 'BOOK_VALUES',
        transactionReconcileDateType: 'TRADE_DATE',
        matchingThresholdsOverride: { value: {} },
        matchingThresholdsSettings: { value: {} },
        positionAutoReconcileOverride: { value: {} },
        positionAutoReconcileSettings: { value: {} },
        transactionAutoReconcileOverride: { value: {} },
        transactionAutoReconcileSettings: { value: {} },
        transactionAutoAcceptCustodianOverride: { value: {} },
        transactionAutoAcceptCustodianSettings: { value: {} },
        positionReconcileDateTypeOverrideInstrumentTypes: [],
    };
}
const payloadRuleSetInput = () => ({ ruleSet: minimalCustodianSetting() });
const payloadUpdateGlobalRule = () => ({ globalRule: { reconcileDate: 'TODAY', updatePriceAtStartOfDay: false } });
const payloadBulkFilters = () => ({ reconDate: '2025-01-01' });
const payloadBulkPositions = () => ({ action: 'GET_COUNT', filters: payloadBulkFilters() });
const payloadBulkMatched   = () => ({ action: 'GET_COUNT', filters: payloadBulkFilters() });
const payloadBulkUnmatched = () => ({ action: 'GET_COUNT', filters: payloadBulkFilters() });
const payloadGetNextTrigger = () => ({ /* currentTrigger опционален */ });
const payloadTxnIds = () => ({ ids: [uuidv4()] });
const payloadRemoveAssignments = () => ({ custodianIds: ['c1'] });
const payloadReplaceAssignments = () => ({ ruleSetId: uuidv4(), custodianIds: ['c1'] });

// --- Positive cases
function makePositiveCases() {
    const rsId = uuidv4();
    const qInput = encodeURIComponent(JSON.stringify({})); // required query object

    return [
        { m: 'GET',   p: `/api-internal/reconciliation/v1/reconciliation-rules` },
        { m: 'GET',   p: `/api-internal/reconciliation/v1/reconciliation-rules/global-rule` },
        { m: 'PATCH', p: `/api-internal/reconciliation/v1/reconciliation-rules/global-rule`, body: payloadUpdateGlobalRule() },

        { m: 'POST',  p: `/api-internal/reconciliation/v1/reconciliation-rules/rule-sets`, body: payloadRuleSetInput() },
        { m: 'GET',   p: `/api-internal/reconciliation/v1/reconciliation-rules/rule-sets/${rsId}` },
        { m: 'PATCH', p: `/api-internal/reconciliation/v1/reconciliation-rules/rule-sets/${rsId}`, body: payloadRuleSetInput() },
        { m: 'DELETE',p: `/api-internal/reconciliation/v1/reconciliation-rules/rule-sets/${rsId}` },

        { m: 'GET',   p: `/api-internal/reconciliation/v1/reconciliation-rules/rule-sets/assignments` },
        { m: 'GET',   p: `/api-internal/reconciliation/v1/reconciliation-rules/rule-sets/${rsId}/assignments` },

        { m: 'POST',  p: `/api-internal/reconciliation/v1/reconciliation-rules/rule-sets/remove-assignments`, body: payloadRemoveAssignments() },
        { m: 'POST',  p: `/api-internal/reconciliation/v1/reconciliation-rules/rule-sets/replace-assignments`, body: payloadReplaceAssignments() },

        { m: 'POST',  p: `/api-internal/reconciliation/v1/ai/get-next-agent-trigger`, body: payloadGetNextTrigger() },

        { m: 'POST',  p: `/api-internal/reconciliation/v1/reconciliation-positions/actions/bulk`, body: payloadBulkPositions() },
        { m: 'POST',  p: `/api-internal/reconciliation/v1/reconciliation-transactions/actions/bulk-matched`, body: payloadBulkMatched() },
        { m: 'POST',  p: `/api-internal/reconciliation/v1/reconciliation-transactions/actions/bulk-unmatched`, body: payloadBulkUnmatched() },

        { m: 'GET',   p: `/api-internal/reconciliation/v1/reconciliation-positions/object-labels?input=${qInput}` },

        { m: 'POST',  p: `/api-internal/reconciliation/v1/reconciliation-positions/transactions/query`, body: payloadTxnIds() },
    ];
}

// --- Negative cases
function makeNegativeCases() {
    const badUUID = 'not-a-uuid';
    const rsId = uuidv4(); // будем использовать валидный для некоторых негативов

    return [
        // 1) Отсутствует заголовок X-Wallarm-Schema-ID
        { title: 'missing schema header', m: 'GET',  p: `/api-internal/reconciliation/v1/reconciliation-rules`, headers: {} },
        { title: 'missing schema header', m: 'POST', p: `/api-internal/reconciliation/v1/reconciliation-rules/rule-sets`, headers: { 'Content-Type': 'application/json' }, body: payloadRuleSetInput() },

        // 2) Неверный X-Wallarm-Schema-ID
        { title: 'wrong schema header', m: 'GET',  p: `/api-internal/reconciliation/v1/reconciliation-rules/global-rule`, headers: H_WRONG },
        { title: 'wrong schema header', m: 'PATCH',p: `/api-internal/reconciliation/v1/reconciliation-rules/global-rule`, headers: H_JSON_WRONG, body: payloadUpdateGlobalRule() },

        // 3) Невалидный UUID в path
        { title: 'invalid uuid in path', m: 'GET',   p: `/api-internal/reconciliation/v1/reconciliation-rules/rule-sets/${badUUID}`, headers: H },
        { title: 'invalid uuid in path', m: 'PATCH', p: `/api-internal/reconciliation/v1/reconciliation-rules/rule-sets/${badUUID}`, headers: H_JSON, body: payloadRuleSetInput() },
        { title: 'invalid uuid in path', m: 'DELETE',p: `/api-internal/reconciliation/v1/reconciliation-rules/rule-sets/${badUUID}`, headers: H },
        { title: 'invalid uuid in path', m: 'GET',   p: `/api-internal/reconciliation/v1/reconciliation-rules/rule-sets/${badUUID}/assignments`, headers: H },

        // 4) Пустой/битый body там, где он обязателен
        { title: 'empty body', m: 'POST',  p: `/api-internal/reconciliation/v1/reconciliation-rules/rule-sets`, headers: H_JSON, body: {} },
        { title: 'empty body', m: 'PATCH', p: `/api-internal/reconciliation/v1/reconciliation-rules/global-rule`, headers: H_JSON, body: {} },
        { title: 'empty body', m: 'PATCH', p: `/api-internal/reconciliation/v1/reconciliation-rules/rule-sets/${rsId}`, headers: H_JSON, body: {} },
        { title: 'empty body', m: 'POST',  p: `/api-internal/reconciliation/v1/ai/get-next-agent-trigger`, headers: H_JSON, body: {} },
        { title: 'empty body', m: 'POST',  p: `/api-internal/reconciliation/v1/reconciliation-positions/actions/bulk`, headers: H_JSON, body: {} },
        { title: 'empty body', m: 'POST',  p: `/api-internal/reconciliation/v1/reconciliation-transactions/actions/bulk-matched`, headers: H_JSON, body: {} },
        { title: 'empty body', m: 'POST',  p: `/api-internal/reconciliation/v1/reconciliation-transactions/actions/bulk-unmatched`, headers: H_JSON, body: {} },
        { title: 'empty body', m: 'POST',  p: `/api-internal/reconciliation/v1/reconciliation-rules/rule-sets/remove-assignments`, headers: H_JSON, body: {} },
        { title: 'empty body', m: 'POST',  p: `/api-internal/reconciliation/v1/reconciliation-rules/rule-sets/replace-assignments`, headers: H_JSON, body: {} },
        { title: 'empty body', m: 'POST',  p: `/api-internal/reconciliation/v1/reconciliation-positions/transactions/query`, headers: H_JSON, body: {} },

        // 5) Плохой query param: input обязателен, отсутствует
        { title: 'missing required query param', m: 'GET', p: `/api-internal/reconciliation/v1/reconciliation-positions/object-labels`, headers: H },

        // 6) Невалидные поля в body: невалидный UUID в ids
        { title: 'invalid uuid in body ids', m: 'POST', p: `/api-internal/reconciliation/v1/reconciliation-positions/transactions/query`, headers: H_JSON, body: { ids: ['not-a-uuid'] } },

        // 7) Отсутствующее required поле в bulk-параметрах (нет filters)
        { title: 'bulk missing filters', m: 'POST', p: `/api-internal/reconciliation/v1/reconciliation-positions/actions/bulk`, headers: H_JSON, body: { action: 'GET_COUNT' } },
        { title: 'bulk missing action',  m: 'POST', p: `/api-internal/reconciliation/v1/reconciliation-transactions/actions/bulk-matched`, headers: H_JSON, body: { filters: payloadBulkFilters() } },
    ];
}

export default function () {
    // --- Positive run
    group('Reconciliation API – positive cases (expect 200 + stub)', () => {
        for (const c of makePositiveCases()) {
            const url = u(c.p);
            const headers = (c.m === 'GET' || c.m === 'DELETE') ? H : H_JSON;
            const res = doRequest(c.m, url, headers, c.body);
            opTrend.add(res.timings.duration, { path: c.p, method: c.m, kind: 'positive' });

            const body = j(res);
            check(res, { [`${c.m} ${c.p} -> 200`]: (r) => r.status === 200 });
            check(body, { [`${c.m} ${c.p} -> expected body`]: (b) => eq(b, EXPECTED) });
            console.log('Response body:', body);
        }
    });

    // --- Negative run
    group('Reconciliation API – negative cases (expect non-200 and/or body != stub)', () => {
        for (const n of makeNegativeCases()) {
            const url = u(n.p);
            const res = doRequest(n.m, url, n.headers ?? H, n.body);
            opTrend.add(res.timings.duration, { path: n.p, method: n.m, kind: 'negative', title: n.title });

            const body = j(res);
            check(res, { [`NEG ${n.m} ${n.p} (${n.title}) -> status != 200`]: (r) => r.status !== 200 });
            check(body, { [`NEG ${n.m} ${n.p} (${n.title}) -> body != stub`]: (b) => !eq(b, EXPECTED) });
            console.log('Response body:', body);
        }
    });
}