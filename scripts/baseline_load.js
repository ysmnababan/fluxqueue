import http from 'k6/http';
import { check } from 'k6';

export const options = {
    stages: [
        { duration: '1m', target: 50 },
        { duration: '3m', target: 50 },
        { duration: '1m', target: 0 },
    ],
    thresholds: {
        http_req_failed: ['rate<0.01'],
        http_req_duration: ['p(95)<1000'],
    },
};

export default function () {
    const res = http.post(
        'http://localhost:8080/api/v1/enqueue',
        JSON.stringify({
            type: 'email.send',
            payload: { to: 'bob@example.com', body: 'Hi Bob' },
            max_retries: 3,
        }),
        { headers: { 'Content-Type': 'application/json' } }
    );

    check(res, {
        'status is 200': (r) => r.status === 200,
    });
}
