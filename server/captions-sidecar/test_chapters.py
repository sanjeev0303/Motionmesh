import sys
import logging
from app.transcribe import generate_chapters

logging.basicConfig(level=logging.INFO)

print("Starting...")
chapters = generate_chapters("This is a long text about nothing " * 50)
print("Chapters:", chapters)
