import { describe, it, expect } from "vitest";
import { fetchStatus } from "../src/services/api";

describe("fetchStatus", () => {
  it("should return healthy status for service", async () => {
    const res = await fetchStatus();
    expect(res.service).toBe("{{repo_name}}");
    expect(res.status).toBe("healthy");
    expect(res.timestamp).toBeDefined();
  });
});
