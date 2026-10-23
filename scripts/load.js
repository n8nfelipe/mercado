import http from "k6/http";
import { check, sleep } from "k6";

export const options = {
  vus: 1,
  duration: "5s",
  thresholds: {
    http_req_failed: ["rate==0"],
    http_req_duration: ["p(95)<1000"],
  },
};

const baseURL = __ENV.BASE_URL || "http://api.mercado.localhost:8080";

export default function () {
  const response = http.get(`${baseURL}/v1/status`);
  check(response, {
    "status is 200": (r) => r.status === 200,
    "body has status": (r) => r.body && r.body.includes('"status"'),
  });
  sleep(0.2);
}
