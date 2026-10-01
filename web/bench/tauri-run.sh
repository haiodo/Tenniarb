#!/bin/sh
# usage: tauri-run.sh <app-binary> <out.json> ; prints startup/RSS facts
B=$1; export BENCH_OUT=$2 BENCH_EXIT=1; rm -f $BENCH_OUT
pgrep -f "WebKit.WebContent|WebKit.GPU|WebKit.Networking" | sort > /tmp/wk_before.txt
S=$(python3 -c 'import time;print(time.time())')
$B > /dev/null 2>&1 &
PID=$!
sleep 5
echo "main RSS KB: $(ps -o rss= -p $PID)"
pgrep -f "WebKit.WebContent|WebKit.GPU|WebKit.Networking" | sort | comm -13 /tmp/wk_before.txt - | while read p; do echo "new webkit pid $p RSS KB $(ps -o rss= -p $p) $(ps -o command= -p $p | sed 's/.*XPCServices\///' | cut -c1-40)"; done
vmmap -summary $PID 2>/dev/null | grep -i "physical footprint"
for i in $(seq 1 90); do kill -0 $PID 2>/dev/null || break; sleep 1; done
E=$(python3 -c 'import time;print(time.time())')
python3 -c "print('total runtime s', round($E-$S,1))"; ls $BENCH_OUT
