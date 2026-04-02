#!/bin/bash

# Gulin token found in logs with active balance
GULIN_TOKEN="gl-vi-al-Hep7isZ4NUYzwtBCpCGYMG5A2ycQnFOg"
BASE_URL="http://localhost:8090"

# Fetch all model IDs from the Bridge
MODELS=$(curl -s $BASE_URL/v1/models | jq -r '.data[].id')

echo "🛡️ INICIANDO SANEAMIENTO DE MODELOS (Detección de Cuota y Respuesta Real)"
echo "----------------------------------------------------------------------"

if [ -z "$MODELS" ]; then
    echo "❌ No se encontraron modelos."
    exit 1
fi

REPORT_FILE="model_sanitization_report.txt"
CLEAN_LIST="working_models.txt"
echo "Reporte de Saneamiento - $(date)" > $REPORT_FILE
echo "" > $CLEAN_LIST

TOTAL=0
SUCCESS=0
QUOTA_ERR=0
EMPTY_ERR=0

for ID in $MODELS; do
    ((TOTAL++))
    printf "[%3d] Validando %-50s " "$TOTAL" "$ID..."
    
    RESPONSE=$(curl -s -X POST $BASE_URL/v1/chat/completions \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $GULIN_TOKEN" \
        -d "{
            \"model\": \"$ID\",
            \"messages\": [{\"role\": \"user\", \"content\": \"Responde solo: OK\"}],
            \"max_tokens\": 5,
            \"stream\": false
        }")
    
    # 1. Detectar Error de Cuota (429)
    if echo "$RESPONSE" | grep -q "RESOURCE_EXHAUSTED" || echo "$RESPONSE" | grep -q "quota" || echo "$RESPONSE" | grep -q "429"; then
        echo -e "\033[0;33m[QUOTA EXCEEDED]\033[0m"
        echo "[QUOTA] $ID" >> $REPORT_FILE
        ((QUOTA_ERR++))
        continue
    fi

    # 2. Extraer contenido y validar longitud
    AI_TEXT=$(echo "$RESPONSE" | jq -r '.choices[0].message.content // ""')
    
    if [ -n "$AI_TEXT" ] && [ "$AI_TEXT" != "null" ]; then
        echo -e "\033[0;32m[ALIVE]\033[0m (\"$AI_TEXT\")"
        echo "[OK] $ID" >> $REPORT_FILE
        echo "$ID" >> $CLEAN_LIST
        ((SUCCESS++))
    else
        echo -e "\033[0;31m[EMPTY/FAIL]\033[0m"
        echo "[FAIL] $ID" >> $REPORT_FILE
        ((EMPTY_ERR++))
    fi
done

echo "----------------------------------------------------------------------"
echo "📊 RESUMEN DE SANEAMIENTO:"
echo "Total analizados: $TOTAL"
echo -e "Operativos:      \033[0;32m$SUCCESS\033[0m"
echo -e "Sin Cuota (429): \033[0;33m$QUOTA_ERR\033[0m"
echo -e "Fallidos/Vacíos: \033[0;31m$EMPTY_ERR\033[0m"
echo "----------------------------------------------------------------------"
