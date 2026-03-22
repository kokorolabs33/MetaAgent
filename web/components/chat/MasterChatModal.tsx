"use client";

interface Props {
  open: boolean;
  onClose: () => void;
}

export function MasterChatModal({ open, onClose }: Props) {
  // V2 placeholder — will be replaced with org-scoped group chat
  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="bg-gray-900 border border-gray-700 rounded-xl p-8 max-w-md">
        <p className="text-gray-300 text-sm mb-4">Chat — coming in V2</p>
        <button
          onClick={onClose}
          className="text-sm text-gray-400 hover:text-white"
        >
          Close
        </button>
      </div>
    </div>
  );
}
