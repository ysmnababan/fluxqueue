import http from 'k6/http';
import { check } from 'k6';

export const options = {
    stages: [
        { duration: '10s', target: 20 },   // normal traffic
        { duration: '5s', target: 500 },   // sudden spike
        { duration: '30s', target: 500 },  // hold spike
        { duration: '30s', target: 20 },   // recovery
        { duration: '10s', target: 0 },
    ],
    thresholds: {
        http_req_failed: ['rate<0.05'],
        http_req_duration: ['p(95)<3000'],
    },
};

export default function () {
    const res = http.post(
        'http://localhost:8080/api/v1/enqueue',
        JSON.stringify({
            type: 'email.send',
            payload: { to:"bob@example.com",body:"<h1>Hello!</h1><p>Thanks for signing up.</p>", subject:"Welcome aboard!!!!"},
            max_retries: 3,
        }),
        { headers: { 'Content-Type': 'application/json' } }
    );

    check(res, {
        'status is 200': (r) => r.status === 200,
    });
}
