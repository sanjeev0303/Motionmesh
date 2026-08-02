export class MotionMeshClient {
  constructor(private apiKey: string, private endpoint: string = "http://localhost:8080") {}

  getApiKey() {
    return this.apiKey;
  }
}
