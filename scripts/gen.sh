#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

(
  cd "${ROOT_DIR}/user-service"
  kitex -module example.com/aim/user-service -service UserService -I "${ROOT_DIR}/idl" "${ROOT_DIR}/idl/user.thrift"
  rm -f main.go handler.go
)
(
  cd "${ROOT_DIR}/auth-service"
  kitex -module example.com/aim/auth-service -service AuthService -I "${ROOT_DIR}/idl" "${ROOT_DIR}/idl/auth.thrift"
  kitex -module example.com/aim/auth-service -I "${ROOT_DIR}/idl" "${ROOT_DIR}/idl/user.thrift"
  rm -f main.go handler.go
)
(
  cd "${ROOT_DIR}/gateway"
  kitex -module example.com/aim/gateway -I "${ROOT_DIR}/idl" "${ROOT_DIR}/idl/auth.thrift"
  kitex -module example.com/aim/gateway -I "${ROOT_DIR}/idl" "${ROOT_DIR}/idl/user.thrift"
  kitex -module example.com/aim/gateway -I "${ROOT_DIR}/idl" "${ROOT_DIR}/idl/chat.thrift"
  kitex -module example.com/aim/gateway -I "${ROOT_DIR}/idl" "${ROOT_DIR}/idl/rag.thrift"
)
(
  cd "${ROOT_DIR}/chat-service"
  kitex -module example.com/aim/chat-service -service ChatService -I "${ROOT_DIR}/idl" "${ROOT_DIR}/idl/chat.thrift"
  kitex -module example.com/aim/chat-service -I "${ROOT_DIR}/idl" "${ROOT_DIR}/idl/user.thrift"
  kitex -module example.com/aim/chat-service -I "${ROOT_DIR}/idl" "${ROOT_DIR}/idl/rag.thrift"
  rm -f main.go handler.go
)
(
  cd "${ROOT_DIR}/rag-service"
  kitex -module example.com/aim/rag-service -service RAGService -I "${ROOT_DIR}/idl" "${ROOT_DIR}/idl/rag.thrift"
  rm -f main.go handler.go
)
