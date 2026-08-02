import os
import logging
from faster_whisper import WhisperModel
from dotenv import load_dotenv

load_dotenv()

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

logger.info("Testing WhisperModel")
try:
    model = WhisperModel("Systran/faster-whisper-tiny.en", device="cpu", compute_type="int8", cpu_threads=2)
    logger.info("Model loaded successfully")
except Exception as e:
    logger.error(f"Failed to load model: {e}")

logger.info("Testing Gemini")
try:
    from langchain_google_genai import ChatGoogleGenerativeAI
    from langchain_core.prompts import ChatPromptTemplate

    api_key = os.getenv("GEMINI_API_KEY")
    if not api_key:
        logger.error("GEMINI_API_KEY is not set!")
    else:
        llm = ChatGoogleGenerativeAI(
            model="gemini-3.6-flash",
            google_api_key=api_key,
            temperature=0.1
        )
        prompt = ChatPromptTemplate.from_messages([
            ("human", "Say hello world")
        ])
        chain = prompt | llm
        res = chain.invoke({})
        logger.info(f"LLM Response: {res.content}")
except Exception as e:
    logger.error(f"Failed Gemini: {e}")
