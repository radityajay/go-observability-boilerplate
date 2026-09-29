import http from "k6/http";
import { check, sleep } from "k6";
import { Rate, Trend } from "k6/metrics";

const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";

const errorRate = new Rate("errors");
const loginDuration = new Trend("login_duration", true);

export const options = {
  stages: [
    { duration: "30s", target: 50 },   // Ramp up
    { duration: "1m", target: 200 },    // Sustained load
    { duration: "30s", target: 500 },   // Spike
    { duration: "30s", target: 0 },     // Ramp down
  ],
  thresholds: {
    http_req_duration: ["p(95)<500"],    // 95% of requests < 500ms
    errors: ["rate<0.01"],               // Error rate < 1%
  },
};

const uniqueEmail = () =>
  `user_${Date.now()}_${Math.random().toString(36).slice(2, 8)}@test.com`;

export default function () {
  const email = uniqueEmail();
  const password = "TestPass123!";

  // Register
  const registerRes = http.post(
    `${BASE_URL}/auth/register`,
    JSON.stringify({ email, password }),
    { headers: { "Content-Type": "application/json" } }
  );
  check(registerRes, { "register 201": (r) => r.status === 201 });
  errorRate.add(registerRes.status !== 201);

  // Login
  const loginRes = http.post(
    `${BASE_URL}/auth/login`,
    JSON.stringify({ email, password }),
    { headers: { "Content-Type": "application/json" } }
  );
  check(loginRes, { "login 200": (r) => r.status === 200 });
  errorRate.add(loginRes.status !== 200);
  loginDuration.add(loginRes.timings.duration);

  if (loginRes.status !== 200) {
    sleep(0.5);
    return;
  }

  const tokens = JSON.parse(loginRes.body);

  // Get profile
  const meRes = http.get(`${BASE_URL}/auth/me`, {
    headers: { Authorization: `Bearer ${tokens.access_token}` },
  });
  check(meRes, { "me 200": (r) => r.status === 200 });
  errorRate.add(meRes.status !== 200);

  // Refresh
  const refreshRes = http.post(
    `${BASE_URL}/auth/refresh`,
    JSON.stringify({ refresh_token: tokens.refresh_token }),
    { headers: { "Content-Type": "application/json" } }
  );
  check(refreshRes, { "refresh 200": (r) => r.status === 200 });

  // Logout
  const logoutRes = http.post(`${BASE_URL}/auth/logout`, null, {
    headers: { Authorization: `Bearer ${tokens.access_token}` },
  });
  check(logoutRes, { "logout 200": (r) => r.status === 200 });

  sleep(0.3);
}

// Health check scenario
export function healthCheck() {
  const res = http.get(`${BASE_URL}/health`);
  check(res, { "health 200": (r) => r.status === 200 });
}
