// doe_v2_account_authorization_validations.test.js
import http from 'k6/http';
import { check, group, sleep } from 'k6';
import { Trend } from 'k6/metrics';

export const options = {
    vus: Number(__ENV.VUS || 1),
    iterations: Number(__ENV.ITERATIONS || 1),
    thresholds: {
        http_req_failed: ['rate==0'],
        http_req_duration: ['p(95)<800'],
        'doe_v2_get_duration': ['p(95)<600'],
        'doe_v2_post_duration': ['p(95)<600'],
    },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8282';
const TEST_DATE = __ENV.TEST_DATE || new Date().toISOString().slice(0, 10);

const postTrend = new Trend('doe_v2_post_duration');
const getTrend  = new Trend('doe_v2_get_duration');

// Общие заголовки (по требованию)
const COMMON_HEADERS_JSON = {
    'Content-Type': 'application/json',
    'X-Wallarm-Schema-ID': '29',
};
const COMMON_HEADERS = {
    'X-Wallarm-Schema-ID': '29',
};

function safeJson(res) {
    try { return res.json(); } catch { return null; }
}
function isString(v) { return typeof v === 'string'; }
function looksLikeUrl(s) { return isString(s) && /^(https?:\/\/|\/)/i.test(s); }

export default function () {
    group('POST /api-internal/custodian-data/v1/account-authorization-validations', () => {
        const endpoint = `${BASE_URL}/api-internal/custodian-data/v1/account-authorization-validations`;
        const payload = { date: TEST_DATE };

        const res = http.post(endpoint, JSON.stringify(payload), {
            headers: COMMON_HEADERS_JSON,
            tags: { service: 'doe-v2', operationId: 'account-authorization-validations-post' },
        });
        postTrend.add(res.timings.duration);

        check(res, {
            'POST status 200': (r) => r.status === 200,
            'POST content-type JSON': (r) => (r.headers['Content-Type'] || '').includes('application/json'),
        });

        const json = safeJson(res);
        check(json, {
            'POST body is object': (j) => j && typeof j === 'object',
            'POST has date': (j) => j && isString(j.date),
            'POST date echoes input': (j) => j && j.date === TEST_DATE,
        });
    });

    sleep(0.2);

    group('GET /api-internal/custodian-data/v1/account-authorization-validations/{date}', () => {
        const endpoint = `${BASE_URL}/api-internal/custodian-data/v1/account-authorization-validations/${encodeURIComponent(TEST_DATE)}`;
        const res = http.get(endpoint, {
            headers: COMMON_HEADERS,
            tags: { service: 'doe-v2', operationId: 'account-authorization-validations-get' },
        });
        getTrend.add(res.timings.duration);

        check(res, {
            'GET status 200': (r) => r.status === 200,
            'GET content-type JSON': (r) => (r.headers['Content-Type'] || '').includes('application/json'),
        });

        const json = safeJson(res);
        check(json, {
            'GET body is object': (j) => j && typeof j === 'object',
            'has failureReportUrl': (j) => j && looksLikeUrl(j.failureReportUrl),
            'has successReportUrl': (j) => j && looksLikeUrl(j.successReportUrl),
            'has summaryReportUrl': (j) => j && looksLikeUrl(j.summaryReportUrl),
        });
    });
}