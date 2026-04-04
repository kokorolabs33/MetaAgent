"use client";

import { useParams, useSearchParams } from "next/navigation";
import { useEffect, useRef, Suspense } from "react";
import { ConversationView } from "@/components/conversation/ConversationView";
import { useConversationStore } from "@/lib/conversationStore";

function ConversationPageInner() {
  const params = useParams();
  const searchParams = useSearchParams();
  const conversationId = params.id as string;
  const initialMessage = searchParams.get("q");
  const sentRef = useRef(false);

  const { activeConversation, sendMessage } = useConversationStore();

  // Send initial message from query param (from EmptyState redirect)
  useEffect(() => {
    if (
      initialMessage &&
      !sentRef.current &&
      activeConversation?.id === conversationId
    ) {
      sentRef.current = true;
      void sendMessage(initialMessage);
    }
  }, [initialMessage, activeConversation, conversationId, sendMessage]);

  return <ConversationView conversationId={conversationId} />;
}

export default function ConversationPage() {
  return (
    <Suspense
      fallback={
        <div className="flex h-full items-center justify-center">
          <div className="size-6 animate-spin rounded-full border-2 border-muted-foreground border-t-transparent" />
        </div>
      }
    >
      <ConversationPageInner />
    </Suspense>
  );
}
