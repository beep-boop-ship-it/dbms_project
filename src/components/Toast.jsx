import React, { useEffect, useMemo, useState } from 'react';

export default function Toast({ message, onDone }) {
  useEffect(() => { if (!message) return; const id = setTimeout(onDone, 2800); return () => clearTimeout(id); }, [message, onDone]);
  return message ? <div className="toast" role="status">{message}</div> : null;
}
