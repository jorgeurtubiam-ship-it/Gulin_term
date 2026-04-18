#!/bin/bash

# Este script resume el estado de los modelos basándose en el último informe de diagnóstico.
REPORT_FILE="verbose_models_report.log"

if [ ! -f "$REPORT_FILE" ]; then
    echo "❌ No se encontró el archivo $REPORT_FILE. Ejecuta primero el diagnóstico."
    exit 1
fi

echo "📊 RESUMEN DE MODELOS OPERATIVOS"
echo "================================="

# Extraer modelos que devolvieron HTTP 200
grep -B 10 "HTTP_STATUS:200" "$REPORT_FILE" | grep -e "--- MODEL:" | sed 's/--- MODEL: \(.*\) ---/\1/' > working_temp.txt

WORKING_COUNT=$(wc -l < working_temp.txt)

if [ "$WORKING_COUNT" -eq 0 ]; then
    echo "⚠️ No se encontraron modelos con respuesta exitosa (200 OK) en el log."
else
    echo "✅ se encontraron $WORKING_COUNT modelos funcionando:"
    cat working_temp.txt
fi

echo -e "\n❌ MODELOS CON ERROR (Resumen):"
grep "HTTP_STATUS:[^2][^0][^0]" "$REPORT_FILE" -B 2 | grep -e "--- MODEL:" | sed 's/--- MODEL: \(.*\) ---/\1/' | head -n 10
echo "... (ver log para más detalles)"

rm working_temp.txt
