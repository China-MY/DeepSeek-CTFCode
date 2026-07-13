#!/usr/bin/env bash
# ctfcode statusline — 显示渗透测试流程状态、子步骤与实时费用
# 读取 stdin JSON payload（包含 model/balance/contextUsed/contextWindow/cwd），输出状态行
# 支持 jq 时解析 JSON，否则使用脚本默认值

read -r INPUT

# 默认值
MODEL="..."
BALANCE=""
PHASE="${PENTEST_PHASE:-idle}"
SUBSTEP="${PENTEST_SUBSTEP:-""

# 尝试解析 JSON
if command -v jq &>/dev/null && [ -n "$INPUT" ]; then
  MODEL=$(echo "$INPUT" | jq -r '.model // "..."' 2>/dev/null)
  BALANCE=$(echo "$INPUT" | jq -r '.balance // ""' 2>/dev/null)
fi

# 渗透流程节点
case "$PHASE" in
  recon)
    FLOW="●🔍→○💥→○📋"
    PHASE_TAG="🔍 RECON"
    ;;
  exploit)
    FLOW="○🔍→●💥→○📋"
    PHASE_TAG="💥 EXPLOIT"
    ;;
  report)
    FLOW="○🔍→○💥→●📋"
    PHASE_TAG="📋 REPORT"
    ;;
  complete)
    FLOW="✅🔍→✅💥→✅📋"
    PHASE_TAG="✅ DONE"
    ;;
  *)
    FLOW="○🔍→○💥→○📋"
    PHASE_TAG="⏳ IDLE"
    ;;
esac

# 子步骤显示
SUB_TAG=""
if [ -n "$SUBSTEP" ]; then
  SUB_TAG="  ${SUBSTEP}"
fi

# 余额显示
BAL_TAG=""
if [ -n "$BALANCE" ]; then
  BAL_TAG="  bal:${BALANCE}"
fi

echo "${PHASE_TAG}${SUB_TAG}  ${FLOW}  |  ${MODEL}${BAL_TAG}"
