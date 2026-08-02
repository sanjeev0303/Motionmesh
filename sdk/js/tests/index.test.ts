import { MotionMeshClient } from "../src/index";

test("client initialization", () => {
  const client = new MotionMeshClient("mot_live_test");
  expect(client.getApiKey()).toBe("mot_live_test");
});
