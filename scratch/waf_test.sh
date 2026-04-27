#!/bin/bash

# Test extremo de caracteres especiales y palabras reservadas
echo "TEST EXTREMO ANTI-WAF"
echo "----------------------"

# Comandos peligrosos
echo "Comando bloqueado 1: sudo su -"
echo "Comando bloqueado 2: rm -rf /etc/configs"
echo "Comando bloqueado 3: systemctl stop firewall"

# Pipes y redirecciones complejas
echo "Tubería compleja: cat /var/log/syslog | grep -i error | awk '{print \$5}' | sort | uniq -c | head -n 10"
echo "Redirección y subshells: (ls -l | grep .sh) > /tmp/scripts_list.txt 2>&1"

# Caracteres de escape y símbolos
echo "Símbolos: \ / | { } [ ] ( ) < > ; : ' \" ! @ # $ % ^ & * - _ + ="
echo "Unicode: │ ¦ ║ ╣ ╗ ╝ ╚ ╔ ╩ ╦ ╠ ═ ╬"

echo "JSON con pipes ficticios:"
echo '{"cmd": "ls | grep test", "args": ["-la", "|", "awk"]}'

echo "----------------------"
echo "FIN DEL TEST"
