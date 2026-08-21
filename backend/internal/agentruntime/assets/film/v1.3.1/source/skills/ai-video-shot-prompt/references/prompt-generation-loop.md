# 提示词到生成结果的证据链

Prompt 版本可被写好，但这不表示模型已运行。GenerationAttempt 必须记录 attempt_id、提示词版本、预期模型/配置证据和 execution_state；没有可访问媒体引用的 GenerationResult 只能是 UNKNOWN。没有媒体的 QC 必须为 NOT_ASSESSABLE，不能 PASS。

失败后记录可定位缺陷（角色漂移、动作、空间、文字、运镜等）和修复假设，然后创建新的 prompt/attempt 版本，`retry_of` 指向旧 Attempt。不得覆盖旧提示词、Attempt 或 Result。
