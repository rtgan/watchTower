---
name: 文件搜索指南
description: 在服务器上按文件名关键词搜索文件。当用户需要查找包含特定关键词的文件、定位图片、文档或任何服务器上的文件时使用。
---

# 文件搜索指南

## 使用场景
- 按关键词查找文件（如查找名称含"哈尔滨"的图片）
- 按扩展名过滤（如只找 jpg/png 图片，或只找 pdf 文档）
- 在指定目录下递归搜索

## 工具：search_file

### 参数说明
- keyword（必填）：文件名关键词，大小写不敏感
- search_path（可选）：搜索根目录，默认 /tmp
- extensions（可选）：扩展名过滤，逗号分隔。常见组合：
  - 图片：jpg,jpeg,png,gif,bmp,webp
  - 文档：pdf,docx,doc,xlsx,xls,pptx
  - 视频：mp4,avi,mov,mkv
  - 日志：log,txt
- max_results（可选）：最大返回数，默认 50

### 使用示例

查找含"哈尔滨"的图片：
- keyword: "哈尔滨"
- search_path: "/data/images"
- extensions: "jpg,jpeg,png,gif,webp"

查找所有 PDF 报告：
- keyword: "report"
- search_path: "/home/user/documents"
- extensions: "pdf"

### 注意事项
- 搜索路径必须是服务器上真实存在的目录
- 关键词匹配的是文件名（不含路径部分），不区分大小写
- 如果结果被截断（truncated=true），可缩小搜索范围或增加 max_results
