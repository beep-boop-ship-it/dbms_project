import React, { useEffect, useMemo, useState } from 'react';

export default function StatCard({ label, value, foot, symbol }) { return <div className="card stat"><div className="stat-top"><span>{label}</span><span className="stat-symbol">{symbol}</span></div><div className="stat-value">{value}</div><div className="stat-foot">{foot}</div></div>; }
