"use client";

import { useEffect, useRef, useState } from "react";
import { Loader2, Send, X, CheckCircle, ChevronRight } from "lucide-react";
import { useTaskHubStore } from "@/lib/store";

interface ChatMessage {
  role: "user" | "assistant";
  content: string;
}

interface SubtaskPreview {
  agent_name: string;
  instruction: string;
  order: number;
}

interface PlanPreview {
  summary: string;
  subtasks: SubtaskPreview[];
}

interface ChatResponse {
  message: string;
  ready_to_execute: boolean;
  plan?: PlanPreview;
}

interface Props {
  open: boolean;
  onClose: () => void;
}

export function MasterChatModal({ open, onClose }: Props) {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [pendingPlan, setPendingPlan] = useState<PlanPreview | null>(null);
  const [dispatching, setDispatching] = useState(false);
  const bottomRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const { submitTask } = useTaskHubStore();

  useEffect(() => {
    if (open) {
      // Greet on open if fresh
      if (messages.length === 0) {
        setMessages([{
          role: "assistant",
          content: "Hi! I'm Master Agent. Describe what you need done and I'll plan the best team to handle it.",
        }]);
      }
      setTimeout(() => inputRef.current?.focus(), 100);
    }
  }, [open, messages.length]);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  const sendMessage = async (text?: string) => {
    const userText = (text ?? input).trim();
    if (!userText || loading) return;

    const newMessages: ChatMessage[] = [...messages, { role: "user", content: userText }];
    setMessages(newMessages);
    setInput("");
    setLoading(true);
    setPendingPlan(null);

    try {
      const res = await fetch(`${process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090"}/api/chat`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ messages: newMessages }),
      });
      const data: ChatResponse = await res.json();

      setMessages((prev) => [...prev, { role: "assistant", content: data.message }]);
      if (data.ready_to_execute && data.plan) {
        setPendingPlan(data.plan);
      }
    } catch {
      setMessages((prev) => [...prev, {
        role: "assistant",
        content: "Sorry, something went wrong. Please try again.",
      }]);
    } finally {
      setLoading(false);
    }
  };

  const handleDispatch = async () => {
    if (!pendingPlan) return;
    setDispatching(true);
    try {
      await submitTask(pendingPlan.summary);
      onClose();
      setMessages([]);
      setPendingPlan(null);
    } finally {
      setDispatching(false);
    }
  };

  const handleClose = () => {
    onClose();
  };

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="relative w-full max-w-2xl mx-4 bg-gray-900 border border-gray-700 rounded-xl shadow-2xl flex flex-col max-h-[80vh]">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-800">
          <div className="flex items-center gap-3">
            <div className="w-8 h-8 rounded-full bg-purple-600 flex items-center justify-center text-white font-bold text-sm">
              M
            </div>
            <div>
              <div className="text-sm font-semibold text-white">Master Agent</div>
              <div className="text-xs text-gray-500">Orchestration Center</div>
            </div>
          </div>
          <button onClick={handleClose} className="text-gray-500 hover:text-gray-300">
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Messages */}
        <div className="flex-1 overflow-y-auto px-6 py-4 space-y-4 min-h-0">
          {messages.map((m, i) => (
            <div key={i} className={`flex ${m.role === "user" ? "justify-end" : "justify-start"}`}>
              <div
                className={`max-w-[85%] rounded-2xl px-4 py-2.5 text-sm leading-relaxed whitespace-pre-wrap ${
                  m.role === "user"
                    ? "bg-purple-600 text-white rounded-br-sm"
                    : "bg-gray-800 text-gray-200 rounded-bl-sm"
                }`}
              >
                {m.content}
              </div>
            </div>
          ))}

          {loading && (
            <div className="flex justify-start">
              <div className="bg-gray-800 rounded-2xl rounded-bl-sm px-4 py-3">
                <Loader2 className="w-4 h-4 animate-spin text-gray-400" />
              </div>
            </div>
          )}

          {/* Plan Preview Card */}
          {pendingPlan && (
            <div className="bg-gray-800/80 border border-purple-500/30 rounded-xl p-4 space-y-3">
              <div className="text-xs font-semibold text-purple-400 uppercase tracking-wider">
                Proposed Plan
              </div>
              <div className="text-sm text-gray-300">{pendingPlan.summary}</div>
              <div className="space-y-2">
                {pendingPlan.subtasks.map((st, i) => (
                  <div key={i} className="flex items-start gap-2 text-sm">
                    <ChevronRight className="w-4 h-4 text-purple-400 mt-0.5 shrink-0" />
                    <div>
                      <span className="text-purple-300 font-medium">{st.agent_name}</span>
                      <span className="text-gray-400 ml-2">{st.instruction.slice(0, 100)}…</span>
                    </div>
                  </div>
                ))}
              </div>
              <div className="flex gap-2 pt-1">
                <button
                  onClick={handleDispatch}
                  disabled={dispatching}
                  className="flex items-center gap-2 bg-purple-600 hover:bg-purple-700 text-white text-sm px-4 py-2 rounded-lg transition-colors disabled:opacity-50"
                >
                  {dispatching ? (
                    <Loader2 className="w-4 h-4 animate-spin" />
                  ) : (
                    <CheckCircle className="w-4 h-4" />
                  )}
                  Confirm & Dispatch
                </button>
                <button
                  onClick={() => { setPendingPlan(null); sendMessage("Let me revise — please adjust the plan."); }}
                  className="text-sm text-gray-400 hover:text-gray-200 px-4 py-2 rounded-lg border border-gray-700 transition-colors"
                >
                  Revise
                </button>
              </div>
            </div>
          )}

          <div ref={bottomRef} />
        </div>

        {/* Input */}
        <div className="px-6 py-4 border-t border-gray-800">
          <div className="flex gap-3">
            <input
              ref={inputRef}
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => { if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); sendMessage(); }}}
              placeholder="Describe your task or reply to Master Agent..."
              className="flex-1 bg-gray-800 border border-gray-700 rounded-lg px-4 py-2.5 text-sm text-gray-100 placeholder:text-gray-500 focus:outline-none focus:border-purple-500"
              disabled={loading}
            />
            <button
              onClick={() => sendMessage()}
              disabled={!input.trim() || loading}
              className="bg-purple-600 hover:bg-purple-700 disabled:opacity-50 text-white rounded-lg px-4 transition-colors"
            >
              <Send className="w-4 h-4" />
            </button>
          </div>
          <div className="text-xs text-gray-600 mt-2">
            Press Enter to send · Master Agent will propose a plan and ask for confirmation before dispatching
          </div>
        </div>
      </div>
    </div>
  );
}
