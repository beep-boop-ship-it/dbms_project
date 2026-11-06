import React, { useEffect, useMemo, useState } from 'react';

export default function Modal({ title, children, onClose, onSubmit, submitLabel = 'Save changes' }) {
  useEffect(() => { const close = e => e.key === 'Escape' && onClose(); window.addEventListener('keydown', close); return () => window.removeEventListener('keydown', close); }, [onClose]);
  return <div className="modal-backdrop" onMouseDown={e => e.target === e.currentTarget && onClose()}><section className="modal card" role="dialog" aria-modal="true" aria-label={title}><div className="modal-head"><div><div className="eyebrow">Campus portal</div><h2>{title}</h2></div><button className="modal-close" onClick={onClose} aria-label="Close">×</button></div><form onSubmit={onSubmit}>{children}<div className="modal-actions"><button type="button" className="secondary" onClick={onClose}>Cancel</button><button className="primary" type="submit">{submitLabel}</button></div></form></section></div>;
}
