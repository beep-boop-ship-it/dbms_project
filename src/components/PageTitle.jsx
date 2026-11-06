import React from 'react';

export default function PageTitle({ eyebrow, title, intro }) { return <><div className="eyebrow">{eyebrow}</div><h1 className="page-title">{title}</h1><p className="page-intro">{intro}</p></>; }
