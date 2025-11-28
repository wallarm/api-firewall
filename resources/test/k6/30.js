// file: document_management_all_paths_with_negative.test.js
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
const H = { 'X-Wallarm-Schema-ID': '30' };
const H_JSON = { ...H, 'Content-Type': 'application/json' };
const H_BAD = { 'X-Wallarm-Schema-ID': '31' };           // неверное значение
const H_JSON_BAD = { ...H_BAD, 'Content-Type': 'application/json' }; // неверное значение + JSON
const EXPECTED = { summary: [{ schema_id: 30, status_code: 200 }] };

const opTrend = new Trend('dm_op_duration_ms');

function t(endpoint) { return `${BASE_URL}${endpoint}`; }
function j(r) { try { return r.json(); } catch { return null; } }
function eq(a, b) { return JSON.stringify(a) === JSON.stringify(b); }
function uuidv4() {
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, c => {
        const r = (Math.random() * 16) | 0, v = c === 'x' ? r : (r & 0x3 | 0x8);
        return v.toString(16);
    });
}

// ========= минимальные payload'ы для позитивных запросов =========
function payloadEmailBatch() {
    return {
        attachments: [],
        bcc: [],
        cc: [],
        contentType: 'HTML',
        distributionType: 'ATTACHMENT',
        id: uuidv4(),
        name: 'batch',
        notifyAnyway: false,
        recipientAttachments: [],
        status: 'DRAFT',
        to: [],
        type: 'MARKETING_CAMPAIGN',
    };
}
function payloadFetchPaginatedQueryRequestV2() {
    return { dataset: 'documents', fields: [{ id: 'id' }], pagination: { first: 1 } };
}
function payloadFetchGroupAndAggregateQueryRequestV2() {
    return { dataset: 'documents', countTotalVisible: false };
}
function payloadCreateClientPortalDocumentInput() {
    return { archived: false, documentName: 'doc', fileName: 'file.pdf' };
}
function payloadEditDocumentsInput() { return {}; }
function payloadDownloadDocumentsInput() { return {}; }
function payloadEditClientPortalDocumentInput() { return { documentName: 'doc-updated' }; }
function payloadAddPortalUsersAccessInput() { return {}; }
function payloadRemovePortalUsersAccessInput() { return {}; }
function payloadAddBatchRecipientAttachmentRequestBody() { return {}; }

// ========= позитивные кейсы по всей спеке =========
function makePositiveCases() {
    const docId = uuidv4();
    const docId2 = uuidv4();
    const batchId = uuidv4();
    const recId = uuidv4();

    return [
        { m: 'PATCH', p: `/api-internal/document-management/v1/documents`, body: payloadEditDocumentsInput() },
        { m: 'POST',  p: `/api-internal/document-management/v1/documents/query`, body: payloadFetchPaginatedQueryRequestV2() },
        { m: 'POST',  p: `/api-internal/document-management/v1/documents/aggregate`, body: payloadFetchGroupAndAggregateQueryRequestV2() },
        { m: 'GET',   p: `/api-internal/document-management/v1/documents/metadata` },
        { m: 'POST',  p: `/api-internal/document-management/v1/documents/download`, body: payloadDownloadDocumentsInput() },
        { m: 'POST',  p: `/api-internal/document-management/v1/documents/download/${docId}` },
        { m: 'PATCH', p: `/api-internal/document-management/v1/documents/${docId}`, body: payloadEditDocumentsInput() },
        { m: 'GET',   p: `/api-internal/document-management/v1/documents/upload-status?documentIds=${docId}` },
        { m: 'GET',   p: `/api-internal/document-management/v1/documents/${docId2}/upload-status` },

        { m: 'GET',   p: `/api-internal/document-management/v1/dataset/documents/metadata` },
        { m: 'POST',  p: `/api-internal/document-management/v1/dataset/documents/paginated`, body: payloadFetchPaginatedQueryRequestV2() },
        { m: 'POST',  p: `/api-internal/document-management/v1/dataset/documents/group-and-aggregate`, body: payloadFetchGroupAndAggregateQueryRequestV2() },

        { m: 'POST',  p: `/api-internal/document-management/v1/documents/portal`, body: payloadCreateClientPortalDocumentInput() },
        { m: 'POST',  p: `/api-internal/document-management/v1/documents/portal/query`, body: payloadFetchPaginatedQueryRequestV2() },
        { m: 'GET',   p: `/api-internal/document-management/v1/documents/portal/metadata` },
        { m: 'POST',  p: `/api-internal/document-management/v1/documents/portal/aggregate`, body: payloadFetchGroupAndAggregateQueryRequestV2() },
        { m: 'PATCH', p: `/api-internal/document-management/v1/documents/portal/${docId}`, body: payloadEditClientPortalDocumentInput() },

        { m: 'POST',  p: `/api-internal/document-management/v1/documents/portal-user-access`, body: payloadAddPortalUsersAccessInput() },
        { m: 'POST',  p: `/api-internal/document-management/v1/documents/portal-user-access/remove`, body: payloadRemovePortalUsersAccessInput() },
        { m: 'POST',  p: `/api-internal/document-management/v1/documents/portal-user-access/notification`, body: payloadAddPortalUsersAccessInput() },

        { m: 'POST',  p: `/api-internal/document-management/v1/email-batches`, body: payloadEmailBatch() },
        { m: 'GET',   p: `/api-internal/document-management/v1/email-batches/metadata` },
        { m: 'POST',  p: `/api-internal/document-management/v1/email-batches/query`, body: payloadFetchPaginatedQueryRequestV2() },
        { m: 'POST',  p: `/api-internal/document-management/v1/email-batches/aggregate`, body: payloadFetchGroupAndAggregateQueryRequestV2() },
        { m: 'GET',   p: `/api-internal/document-management/v1/email-batches/${batchId}` },
        { m: 'POST',  p: `/api-internal/document-management/v1/email-batches/${batchId}`, body: payloadEmailBatch() },
        { m: 'POST',  p: `/api-internal/document-management/v1/email-batches/${batchId}/send`, body: payloadEmailBatch() },
        { m: 'GET',   p: `/api-internal/document-management/v1/email-batches/${batchId}/preview` },
        { m: 'POST',  p: `/api-internal/document-management/v1/email-batches/${batchId}/document-recipients`, body: payloadAddBatchRecipientAttachmentRequestBody() },
        { m: 'DELETE',p: `/api-internal/document-management/v1/email-batches/${batchId}/document-recipients` },
        { m: 'DELETE',p: `/api-internal/document-management/v1/email-batches/${batchId}/document-recipients/${recId}` },

        { m: 'POST',  p: `/api-internal/document-management/v1/email-recipients/query`, body: payloadFetchPaginatedQueryRequestV2() },
        { m: 'POST',  p: `/api-internal/document-management/v1/email-recipients/aggregate`, body: payloadFetchGroupAndAggregateQueryRequestV2() },
        { m: 'GET',   p: `/api-internal/document-management/v1/email-recipients/metadata` },

        { m: 'POST',  p: `/api-internal/document-management/v1/document-recipients/query`, body: payloadFetchPaginatedQueryRequestV2() },
        { m: 'POST',  p: `/api-internal/document-management/v1/document-recipients/aggregate`, body: payloadFetchGroupAndAggregateQueryRequestV2() },
        { m: 'GET',   p: `/api-internal/document-management/v1/document-recipients/metadata` },
        { m: 'POST',  p: `/api-internal/document-management/v1/document-recipients/paginated`, body: payloadFetchPaginatedQueryRequestV2() },

        { m: 'GET',   p: `/api-internal/document-management/v1/dataset/document-recipients/metadata` },
        { m: 'POST',  p: `/api-internal/document-management/v1/dataset/document-recipients/paginated`, body: payloadFetchPaginatedQueryRequestV2() },
        { m: 'POST',  p: `/api-internal/document-management/v1/dataset/document-recipients/group-and-aggregate`, body: payloadFetchGroupAndAggregateQueryRequestV2() },
    ];
}

// ========= негативные кейсы =========
// - отсутствие заголовка X-Wallarm-Schema-ID
// - неверный X-Wallarm-Schema-ID
// - пустые/некорректные тела для эндпоинтов с обязательным body
// - невалидные UUID в path
// - отсутствие обязательных query параметров
function makeNegativeCases() {
    const badUUID = 'not-a-uuid';

    return [
        // Нет заголовка X-Wallarm-Schema-ID
        { title: 'missing header', m: 'GET',  p: `/api-internal/document-management/v1/documents/metadata`, headers: {} },
        { title: 'missing header', m: 'POST', p: `/api-internal/document-management/v1/documents/query`, headers: { 'Content-Type': 'application/json' }, body: {} },

        // Неверный заголовок X-Wallarm-Schema-ID
        { title: 'wrong schema id', m: 'GET',  p: `/api-internal/document-management/v1/email-batches/metadata`, headers: H_BAD },
        { title: 'wrong schema id', m: 'POST', p: `/api-internal/document-management/v1/email-batches`, headers: H_JSON_BAD, body: {} },

        // Пустое тело там, где нужен обязательный body
        { title: 'empty body', m: 'POST', p: `/api-internal/document-management/v1/dataset/documents/paginated`, headers: H_JSON, body: {} },
        { title: 'empty body', m: 'POST', p: `/api-internal/document-management/v1/documents/aggregate`, headers: H_JSON, body: {} },
        { title: 'empty body', m: 'POST', p: `/api-internal/document-management/v1/documents/portal`, headers: H_JSON, body: {} },

        // Невалидный UUID в path
        { title: 'invalid uuid', m: 'GET',  p: `/api-internal/document-management/v1/email-batches/${badUUID}`, headers: H },
        { title: 'invalid uuid', m: 'PATCH',p: `/api-internal/document-management/v1/documents/${badUUID}`, headers: H_JSON, body: {} },
        { title: 'invalid uuid', m: 'GET',  p: `/api-internal/document-management/v1/documents/${badUUID}/upload-status`, headers: H },

        // Отсутствует обязательный query
        { title: 'missing query', m: 'GET',  p: `/api-internal/document-management/v1/documents/upload-status`, headers: H },

        // Неверный метод (например, DELETE без id where path requires id)
        { title: 'wrong method shape', m: 'DELETE', p: `/api-internal/document-management/v1/email-batches/document-recipients`, headers: H }, // нет batchId в path
    ];
}

function doRequest(method, url, headers, body) {
    if (method === 'GET')   return http.get(url, { headers });
    if (method === 'DELETE')return http.del(url, null, { headers });
    if (method === 'POST')  return http.post(url, body ? JSON.stringify(body) : '', { headers });
    if (method === 'PATCH') return http.patch(url, body ? JSON.stringify(body) : '', { headers });
    throw new Error(`Unsupported method ${method}`);
}

export default function () {
    const positives = makePositiveCases();

    group('Document Management API – positive cases (expect 200 + stub body)', () => {
        for (const c of positives) {
            const url = t(c.p);
            const headers = (c.m === 'GET' || c.m === 'DELETE') ? H : H_JSON;
            const res = doRequest(c.m, url, headers, c.body);

            opTrend.add(res.timings.duration, { path: c.p, method: c.m, kind: 'positive' });

            const body = j(res);
            check(res, { [`${c.m} ${c.p} -> status 200`]: (r) => r.status === 200 });
            check(body, { [`${c.m} ${c.p} -> expected stub`]: (b) => eq(b, EXPECTED) });
        }
    });

    const negatives = makeNegativeCases();

    group('Document Management API – negative cases (expect non-200 and body != stub)', () => {
        for (const n of negatives) {
            const url = t(n.p);
            const res = doRequest(n.m, url, n.headers ?? H, n.body);

            opTrend.add(res.timings.duration, { path: n.p, method: n.m, kind: 'negative', title: n.title });

            const body = j(res);
            check(res, { [`NEG ${n.m} ${n.p} (${n.title}) -> status != 200`]: (r) => r.status !== 200 });
            check(body, { [`NEG ${n.m} ${n.p} (${n.title}) -> body != stub`]: (b) => !eq(b, EXPECTED) });
        }
    });
}