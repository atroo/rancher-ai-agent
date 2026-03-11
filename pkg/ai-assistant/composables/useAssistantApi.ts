import { ref } from 'vue';

/**
 * Vercel AI SDK UI Message Stream Protocol v1 event types.
 * Subset relevant to our assistant.
 */
export interface StreamEvent {
  type: string;
  [key: string]: any;
}

export interface ToolCall {
  toolCallId: string;
  toolName: string;
  input: any;
  output?: any;
  error?: string;
  status: 'pending' | 'running' | 'completed' | 'error';
}

export interface StreamState {
  text: string;
  toolCalls: ToolCall[];
  isStreaming: boolean;
  error: string | null;
  finishReason: string | null;
}

/**
 * Composable for SSE-based streaming chat with the AI assistant backend.
 * Implements the Vercel AI SDK UI Message Stream Protocol v1 (client-side).
 *
 * The backend responds to POST requests with an SSE stream:
 *   data: {"type":"start",...}\n\n
 *   data: {"type":"text-delta",...}\n\n
 *   ...
 *   data: [DONE]\n\n
 */
export function useAssistantApi(clusterId: string) {
  const state = ref<StreamState>({
    text:         '',
    toolCalls:    [],
    isStreaming:  false,
    error:        null,
    finishReason: null,
  });

  let abortController: AbortController | null = null;
  let sessionId: string | null = null;

  /**
   * Send a message and stream the response via SSE.
   * Automatically maintains session ID across messages for conversation memory.
   */
  async function send(
    message: string,
    context?: { clusterId?: string; namespace?: string; resourceType?: string; resourceName?: string },
  ): Promise<void> {
    // Reset state for new message
    state.value = {
      text:         '',
      toolCalls:    [],
      isStreaming:  true,
      error:        null,
      finishReason: null,
    };

    abortController = new AbortController();

    const backendUrl = `/k8s/clusters/${ clusterId }/api/v1/namespaces/cattle-ai-assistant/services/http:ai-assistant-backend:8080/proxy/api/v1/chat`;

    try {
      const response = await fetch(backendUrl, {
        method:  'POST',
        headers: { 'Content-Type': 'application/json' },
        body:    JSON.stringify({ message, context, sessionId }),
        signal:  abortController.signal,
      });

      if (!response.ok) {
        throw new Error(`Backend returned ${ response.status }`);
      }

      const reader = response.body?.getReader();

      if (!reader) {
        throw new Error('Response body is not readable');
      }

      const decoder = new TextDecoder();
      let buffer = '';

      while (true) {
        const { done, value } = await reader.read();

        if (done) {
          break;
        }

        buffer += decoder.decode(value, { stream: true });

        // Parse SSE lines: each event is "data: {json}\n\n"
        const lines = buffer.split('\n\n');

        // Keep the last incomplete chunk in the buffer
        buffer = lines.pop() || '';

        for (const line of lines) {
          const trimmed = line.trim();

          if (!trimmed.startsWith('data: ')) {
            continue;
          }

          const payload = trimmed.slice(6); // strip "data: "

          if (payload === '[DONE]') {
            state.value.isStreaming = false;

            return;
          }

          try {
            const event: StreamEvent = JSON.parse(payload);

            handleEvent(event);
          } catch {
            console.warn('Failed to parse SSE event:', payload);
          }
        }
      }

      state.value.isStreaming = false;
    } catch (err: any) {
      if (err.name === 'AbortError') {
        state.value.isStreaming = false;

        return;
      }
      state.value.error = err.message || 'Failed to reach AI assistant';
      state.value.isStreaming = false;
    }
  }

  function handleEvent(event: StreamEvent) {
    switch (event.type) {
    case 'start':
      if (event.sessionId) {
        sessionId = event.sessionId;
      }
      break;

    case 'text-delta':
      state.value.text += event.delta;
      break;

    case 'tool-input-start':
      state.value.toolCalls.push({
        toolCallId: event.toolCallId,
        toolName:   event.toolName,
        input:      null,
        status:     'pending',
      });
      break;

    case 'tool-input-available': {
      const tc = findToolCall(event.toolCallId);

      if (tc) {
        tc.input = event.input;
        tc.status = 'running';
      }
      break;
    }

    case 'tool-output-available': {
      const tc = findToolCall(event.toolCallId);

      if (tc) {
        tc.output = event.output;
        tc.status = 'completed';
      }
      break;
    }

    case 'tool-output-error': {
      const tc = findToolCall(event.toolCallId);

      if (tc) {
        tc.error = event.errorText;
        tc.status = 'error';
      }
      break;
    }

    case 'error':
      state.value.error = event.errorText;
      break;

    case 'finish':
      state.value.finishReason = event.finishReason;
      break;

      // start, start-step, finish-step, text-start, text-end
      // are structural — no UI state changes needed
    }
  }

  function findToolCall(toolCallId: string): ToolCall | undefined {
    return state.value.toolCalls.find((tc) => tc.toolCallId === toolCallId);
  }

  function abort() {
    abortController?.abort();
  }

  function resetSession() {
    sessionId = null;
  }

  return {
    state,
    send,
    abort,
    resetSession,
  };
}
