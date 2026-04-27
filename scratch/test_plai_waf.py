import requests
import json

def surgical_curl_test():
    endpoint = "https://plai-api-core.cencosud.ai/api/assistant"
    agent_id = "69e44065f9b8bce2d1a4dda2"
    api_key = "TX9LQsu18igdWZYXXVPD3qHqDzva60Oc5OSgcN3YUiZPB6fO7Y1Dhe7ZhXzxGEo2"
    
    headers = {
        "Content-Type": "application/json",
        "x-api-key": api_key,
        "x-agent-id": agent_id
    }

    test_cases = [
        "curl -s https://docs.dremio.com | grep -i dataset", # El que falla
        "curl -s https://docs.dremio.com",                   # Curl solo
        "grep -i dataset",                                   # Grep solo
        "curl -s https://docs.dremio.com │ grep -i dataset", # Con Pipe de Caja
        "curl -s https://docs.dremio.com |grep -i dataset",  # Pegado
    ]

    print("--- TEST QUIRÚRGICO DE CURL + PIPE ---")
    
    for content in test_cases:
        payload = {"input": content}
        print(f"Probando: '{content}' -> ", end="", flush=True)
        
        try:
            response = requests.post(endpoint, headers=headers, json=payload, timeout=15)
            if response.status_code == 403:
                print("BLOQUEADO (403)")
            elif response.status_code == 201:
                print("OK")
            else:
                print(f"Error {response.status_code}")
        except Exception as e:
            print(f"Error: {e}")

if __name__ == "__main__":
    surgical_curl_test()
