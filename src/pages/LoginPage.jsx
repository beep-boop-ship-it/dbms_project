import React, { useState } from 'react';
import CheckPattern from '../components/CheckPattern.jsx';

const demoAccounts = [
  { role: 'student', label: 'Student', username: '2024BCS1021' },
  { role: 'teacher', label: 'Faculty', username: 'nisha@northstar.edu' },
  { role: 'management', label: 'Management', username: 'admin@northstar.edu' },
];

export default function Login({ onLogin, busy, error }) {
  const [role, setRole] = useState('student');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');

  const fillDemoAccount = account => {
    setRole(account.role);
    setUsername(account.username);
    setPassword('demo1234');
  };

  return <div className="login-wrap">
    <CheckPattern />
    <div className="login-brand">
      <div className="brand-mark">M</div>
      <div><strong>myriad</strong><small>TECHNICAL COLLEGE</small></div>
    </div>
    <main className="login-layout">
      <aside className="demo-access" aria-labelledby="demo-access-title">
        <div className="demo-access-head">
          <span className="demo-access-tag">DEMO ACCESS</span>
          <span className="demo-access-note">Temporary accounts</span>
        </div>
        <h2 id="demo-access-title">Use a demo account</h2>
        <p>Choose a role below to fill in its sign-in details.</p>
        <div className="demo-account-list">
          {demoAccounts.map(account => <button
            className={`demo-account ${role === account.role && username === account.username ? 'selected' : ''}`}
            key={account.role}
            type="button"
            onClick={() => fillDemoAccount(account)}
            aria-label={`Use ${account.label} demo account, ${account.username}, password demo1234`}
          >
            <span className="demo-account-role">{account.label}</span>
            <span className="demo-account-credentials"><b>{account.username}</b><small>Password: <strong>demo1234</strong></small></span>
            <span className="demo-account-action">Use account <span aria-hidden="true">↗</span></span>
          </button>)}
        </div>
        <div className="demo-access-foot"><span className="demo-status-dot"/> Demo data resets when the server restarts.</div>
      </aside>

      <form className="login-card card" onSubmit={e => { e.preventDefault(); onLogin({ username, password, role }); }}>
        <div className="eyebrow">Campus access / 01</div>
        <h1>Welcome back.</h1>
        <p className="page-intro">Sign in to open your campus workspace.</p>
        <label className="login-label" htmlFor="login-id">Institute email or ID</label>
        <input className="login-input" id="login-id" autoComplete="username" value={username} onChange={e => setUsername(e.target.value)} required placeholder="Institute email or ID" />
        <label className="login-label" htmlFor="login-password">Password</label>
        <input className="login-input" id="login-password" type="password" autoComplete="current-password" value={password} onChange={e => setPassword(e.target.value)} required placeholder="Enter your password" />
        <label className="login-label" htmlFor="login-role">Sign in as</label>
        <select className="login-input" id="login-role" value={role} onChange={e => setRole(e.target.value)}>
          <option value="student">Student</option>
          <option value="teacher">Faculty / Teacher</option>
          <option value="management">Management</option>
        </select>
        {error && <div className="form-error" role="alert">{error}</div>}
        <button className="primary login-submit" disabled={busy}>{busy ? 'Signing in…' : 'Sign in to portal'}<span aria-hidden="true"> →</span></button>
        <div className="login-demo">Demo environment · Myriad Technical College</div>
      </form>
    </main>
  </div>;
}
