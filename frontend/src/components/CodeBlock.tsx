import { useState } from "react";

export function CodeBlock({ children }: { children: string }) {
  const [copied, setCopied] = useState(false);

  function handleCopy() {
    navigator.clipboard.writeText(children).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  }

  return (
    <div className="code-block-wrap">
      <button type="button" className="copy-btn" onClick={handleCopy}>
        {copied ? "已复制" : "复制"}
      </button>
      <pre className="json-block">{children}</pre>
    </div>
  );
}
