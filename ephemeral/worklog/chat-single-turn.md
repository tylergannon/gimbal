finding: Claude wrapped schema-free text turns in its private structured completion envelope, making native chat generate prose and then another completion tool step. Schema-free turns should use the native text result.
decision: Hide StructuredOutput only in the conversation presentation; retain canonical events and ordinary tool calls for inspection.
