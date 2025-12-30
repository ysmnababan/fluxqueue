import http from 'k6/http';
import { check } from 'k6';

export let options = {
    stages: [
        // Ramp up to moderate load
        { duration: '30s', target: 100 },
        { duration: '30s', target: 200 },
        // Push into stress territory
        { duration: '1m', target: 300 },
        { duration: '1m', target: 500 },
        { duration: '1m', target: 800 },
        // Ramp down
        { duration: '30s', target: 0 },
    ],
    thresholds: {
        // k6 will mark the test as failed if these are violated:
        http_req_failed: ['rate<0.05'],     // < 5% failed
        http_req_duration: ['p(95)<2000'],  // 95% under 2s
    },
};

export default function () {
    const url = 'http://localhost:8080/api/v1/enqueue';

    const payload = JSON.stringify({
        type: "email.send",
        payload: { to: "bob@example.com", body: "Hi Bob" },
        max_retries: 3,
    });

    const params = {
        headers: { 'Content-Type': 'application/json' },
    };

    const res = http.post(url, payload, params);

    check(res, {
        'status is 200': (r) => r.status === 200,
    });
}
