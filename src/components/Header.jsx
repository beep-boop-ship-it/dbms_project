import React from 'react';
import { TODAY } from '../constants.js';

function IconButton({ children, ...props }) { return <button className="icon-btn" {...props}>{children}</button>; }
export default function Header({ page, onLogout, onMenu }) {
  return <header className="topbar"><div className="header-left"><button className="icon-btn mobile-menu" onClick={onMenu} aria-label="Open menu">☰</button><div className="crumb">Campus portal <span>/</span> <b>{page}</b></div></div><div className="top-actions"><span className="date">{TODAY}</span><IconButton title="Sign out" aria-label="Sign out" onClick={onLogout}>↪</IconButton></div></header>;
}
