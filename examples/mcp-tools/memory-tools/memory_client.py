import scriptling.ai as ai
import scriptling.ai.memory as memory
import scriptling.runtime.kv as kv
import os

def open_memory():
    ai_base_url = os.getenv("SCRIPTLING_AI_BASE_URL", "")
    ai_model = os.getenv("SCRIPTLING_AI_MODEL", "")
    ai_token = os.getenv("SCRIPTLING_AI_TOKEN", "")
    ai_provider = os.getenv("SCRIPTLING_AI_PROVIDER", "openai")

    client = ai.Client(ai_base_url, api_key=ai_token, provider=ai_provider) if ai_base_url and ai_model else None

    db = kv.open(os.getenv("SCRIPTLING_MEMORY_DB", "./memory-db"))
    return memory.new(db, ai_client=client, model=ai_model)

open_memory
