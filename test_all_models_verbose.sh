#!/bin/bash

# Configuration
MODELS_JSON="/Users/lordzero1/IA_LoRdZeRo/Gulin_Bridge/models.json"
DOTENV="/Users/lordzero1/IA_LoRdZeRo/Gulin_Bridge/.env"
REPORT_FILE="/Users/lordzero1/IA_LoRdZeRo/Gulin_Agent/waveterm/verbose_models_report.log"

# Load Keys from .env
# We use a helper to extract keys without showing them in logs usually, but here we need them for curl
OPENAI_KEY=$(grep "OPENAI_MASTER_KEY" "$DOTENV" | cut -d'=' -f2)
DEEPSEEK_KEY=$(grep "DEEPSEEK_MASTER_KEY" "$DOTENV" | cut -d'=' -f2)
ANTHROPIC_KEY=$(grep "ANTHROPIC_MASTER_KEY" "$DOTENV" | cut -d'=' -f2)
GOOGLE_KEY=$(grep "GOOGLE_MASTER_KEY" "$DOTENV" | cut -d'=' -f2)

echo "🛡️ INICIANDO DIAGNÓSTICO MASIVO DE MODELOS (Raw Response)" > "$REPORT_FILE"
echo "--------------------------------------------------------" >> "$REPORT_FILE"

# Extract models and providers
jq -r '.models[] | "\(.name)|\(.provider)"' "$MODELS_JSON" > models_temp.txt

COUNT=0
while IFS='|' read -r MODEL PROVIDER; do
    ((COUNT++))
    echo "[$COUNT] Probando $MODEL ($PROVIDER)..."
    echo "--- MODEL: $MODEL ($PROVIDER) ---" >> "$REPORT_FILE"

    IF_PROVIDER="$PROVIDER"
    if [ "$IF_PROVIDER" == "null" ]; then
        if [[ "$MODEL" == models/gemini-* || "$MODEL" == models/gemma-* || "$MODEL" == models/imagen-* || "$MODEL" == models/veo-* || "$MODEL" == models/aqa ]]; then
            IF_PROVIDER="google"
        fi
    fi

    RESPONSE=""
    case "$IF_PROVIDER" in
        openai)
            RESPONSE=$(curl -s -w "\nHTTP_STATUS:%{http_code}\n" -X POST https://api.openai.com/v1/chat/completions \
              -H "Content-Type: application/json" \
              -H "Authorization: Bearer $OPENAI_KEY" \
              -d "{
                \"model\": \"$MODEL\",
                \"messages\": [{\"role\": \"user\", \"content\": \"hola como estas?\"}]
              }")
            ;;
        deepseek)
            RESPONSE=$(curl -s -w "\nHTTP_STATUS:%{http_code}\n" -X POST https://api.deepseek.com/v1/chat/completions \
              -H "Content-Type: application/json" \
              -H "Authorization: Bearer $DEEPSEEK_KEY" \
              -d "{
                \"model\": \"$MODEL\",
                \"messages\": [{\"role\": \"user\", \"content\": \"hola como estas?\"}]
              }")
            ;;
        anthropic)
            RESPONSE=$(curl -s -w "\nHTTP_STATUS:%{http_code}\n" -X POST https://api.anthropic.com/v1/messages \
              -H "Content-Type: application/json" \
              -H "x-api-key: $ANTHROPIC_KEY" \
              -H "anthropic-version: 2023-06-01" \
              -d "{
                \"model\": \"$MODEL\",
                \"max_tokens\": 1024,
                \"messages\": [{\"role\": \"user\", \"content\": \"hola como estas?\"}]
              }")
            ;;
        google)
            G_MODEL=$(echo "$MODEL" | sed 's/^models\///')
            RESPONSE=$(curl -s -w "\nHTTP_STATUS:%{http_code}\n" -X POST "https://generativelanguage.googleapis.com/v1beta/models/$G_MODEL:generateContent?key=$GOOGLE_KEY" \
              -H "Content-Type: application/json" \
              -d "{
                \"contents\": [{\"parts\": [{\"text\": \"hola como estas?\"}]}]
              }")
            ;;
        *)
            RESPONSE="SKIP: Provider $PROVIDER unknown or AUTO"
            ;;
    esac

    echo "$RESPONSE" >> "$REPORT_FILE"
    echo -e "\n--------------------------------------------------------\n" >> "$REPORT_FILE"
    
    STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS" | cut -d':' -f2)
    echo "   Resultado: HTTP $STATUS"
done < models_temp.txt

rm models_temp.txt
echo "✅ Diagnóstico completado. Informe guardado en $REPORT_FILE"
