import os
import json
import logging
from fastapi.testclient import TestClient
from app.main import app

# Increase logging level to see what is happening
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

client = TestClient(app)

def test_health():
    logger.info("Testing /health endpoint")
    response = client.get("/health")
    assert response.status_code == 200
    assert response.json() == {"status": "ok"}
    logger.info("Health check passed.")

def test_transcribe_endpoint():
    logger.info("Testing /transcribe endpoint")
    file_path = "sample.flac"
    if not os.path.exists(file_path):
        raise FileNotFoundError(f"{file_path} not found. Please ensure the test audio file is present.")
    
    from unittest.mock import patch
    from app.transcribe import ChapterInfo
    
    with open(file_path, "rb") as f, patch("app.transcribe.generate_chapters") as mock_gen_chapters:
        mock_gen_chapters.return_value = [
            ChapterInfo(start_time_seconds=0, title="Intro", position=0),
            ChapterInfo(start_time_seconds=5, title="Main Content", position=1)
        ]
        # include_chapters must be sent as form data
        response = client.post(
            "/transcribe",
            files={"audio": ("sample.flac", f, "audio/flac")},
            data={"include_chapters": "true"}
        )
    
    if response.status_code != 200:
        logger.error(f"Error response: {response.text}")
    assert response.status_code == 200, f"Expected 200, got {response.status_code}"
    
    data = response.json()
    logger.info("Response JSON received successfully.")
    print(json.dumps(data, indent=2))
    
    # Assertions to ensure the response format is as expected
    assert "transcript_text" in data
    assert "vtt" in data
    assert "segments" in data
    assert "chapters" in data
    
    assert len(data["transcript_text"]) > 10, "Transcript text seems too short"
    assert len(data["chapters"]) > 0, "No chapters were generated"
    logger.info("All assertions passed for /transcribe endpoint.")

if __name__ == "__main__":
    test_health()
    test_transcribe_endpoint()
    print("\n--- All tests passed successfully! ---")
