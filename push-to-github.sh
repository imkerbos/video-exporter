#!/bin/bash

# 推送代码到 GitHub 的脚本
# 使用方法: ./push-to-github.sh <repository-name>
# 例如: ./push-to-github.sh video-exporter

REPO_NAME=${1:-video-exporter}
GITHUB_USER="imkerbos"
REPO_URL="https://github.com/${GITHUB_USER}/${REPO_NAME}.git"

echo "🚀 准备推送代码到 GitHub..."
echo "📦 仓库名称: ${REPO_NAME}"
echo "🔗 仓库地址: ${REPO_URL}"
echo ""

# 检查是否已设置远程仓库
if git remote get-url origin > /dev/null 2>&1; then
    echo "⚠️  远程仓库已存在，正在更新..."
    git remote set-url origin ${REPO_URL}
else
    echo "➕ 添加远程仓库..."
    git remote add origin ${REPO_URL}
fi

# 确保分支名为 main
echo "🔄 设置分支为 main..."
git branch -M main

echo ""
echo "📤 推送代码到 GitHub..."
echo "   如果这是第一次推送，请确保已在 GitHub 上创建了仓库: ${REPO_URL}"
echo ""

# 推送代码
git push -u origin main

if [ $? -eq 0 ]; then
    echo ""
    echo "✅ 代码已成功推送到 GitHub!"
    echo "🌐 访问地址: ${REPO_URL}"
else
    echo ""
    echo "❌ 推送失败！"
    echo "   请确保："
    echo "   1. 已在 GitHub 上创建了仓库: ${REPO_URL}"
    echo "   2. 已配置 GitHub 认证（SSH key 或 Personal Access Token）"
    echo "   3. 有推送权限"
fi

