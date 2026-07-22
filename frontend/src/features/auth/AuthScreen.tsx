// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Auth (OpenRisk.dc.html §6.1): split-screen 45/55 — animated orbit + Schneier
// quote on the left, Login / Register / MFA forms on the right. The Login form is
// wired to the real auth store; Register/MFA are the design flow.

import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import { Eye, EyeOff, Lock, Sun, Moon } from 'lucide-react';
import { useAuthStore } from '../../hooks/useAuthStore';
import { useUIStore } from '../../store/uiStore';
import { useUIStrings } from '../../shared/uiStrings';
import { OpenRiskLogo } from '../../shared/Logo';

type View = 'login' | 'register' | 'mfa';

const ORBIT_NODES: [string, number, number][] = [
  ['#ff453a', 20, 40], ['#ff9f0a', 210, 70], ['#30d158', 40, 200], ['#64d2ff', 200, 190], ['#7c6cff', 120, 10],
];

// Real, attributed quotes on cybersecurity, risk and science — rotated on the
// login hero to keep the sign-in screen alive without inventing anything.
const QUOTES: [string, string][] = [
  ['Security is a process, not a product.', 'Bruce Schneier'],
  ['The only truly secure system is one that is powered off, cast in a block of concrete and sealed in a lead-lined room with armed guards.', 'Gene Spafford'],
  ['Amateurs hack systems, professionals hack people.', 'Bruce Schneier'],
  ['There are two types of companies: those that have been hacked, and those that don’t yet know they have been hacked.', 'John Chambers'],
  ['It takes 20 years to build a reputation and a few minutes of cyber-incident to ruin it.', 'Stéphane Nappo'],
  ['Complexity is the worst enemy of security.', 'Bruce Schneier'],
  ['Given enough eyeballs, all bugs are shallow.', 'Linus’s Law — Eric S. Raymond'],
  ['In God we trust. All others must bring data.', 'W. Edwards Deming'],
  ['Risk comes from not knowing what you’re doing.', 'Warren Buffett'],
  ['An ounce of prevention is worth a pound of cure.', 'Benjamin Franklin'],
  ['The measure of intelligence is the ability to change.', 'Albert Einstein'],
  ['What we anticipate seldom occurs; what we least expect generally happens.', 'Benjamin Disraeli'],
];

export function AuthScreen({ initialView = 'login' }: { initialView?: View }) {
  const [view, setView] = useState<View>(initialView);
  const L = useUIStrings();
  const theme = useUIStore((s) => s.theme);
  const toggleTheme = useUIStore((s) => s.toggleTheme);

  const [qi, setQi] = useState(() => Math.floor(Math.random() * QUOTES.length));
  const [qShow, setQShow] = useState(true);
  useEffect(() => {
    const t = setInterval(() => {
      setQShow(false);
      setTimeout(() => { setQi((i) => (i + 1) % QUOTES.length); setQShow(true); }, 350);
    }, 7000);
    return () => clearInterval(t);
  }, []);
  const [quote, author] = QUOTES[qi];

  return (
    <div className="flex w-full relative" style={{ height: '100vh' }}>
      {/* Theme toggle — available before sign-in */}
      <button
        onClick={toggleTheme}
        className="absolute top-5 right-5 z-10 w-10 h-10 rounded-[11px] flex items-center justify-center text-ink-muted hover:text-ink transition-colors"
        style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}
        title={theme === 'dark' ? 'Light theme' : 'Dark theme'}
        aria-label="Toggle theme"
      >
        {theme === 'dark' ? <Sun size={18} /> : <Moon size={18} />}
      </button>
      {/* left */}
      <div className="relative overflow-hidden flex-col justify-between p-11 hidden md:flex" style={{ flex: '0 0 45%', background: 'linear-gradient(150deg,#0a0b12,#111225)' }}>
        <div className="absolute rounded-full" style={{ top: '-15%', right: '-10%', width: 420, height: 420, background: 'radial-gradient(circle,var(--accent-glow),transparent 70%)', filter: 'blur(30px)', opacity: 0.5 }} />
        <div className="absolute rounded-full" style={{ bottom: '0%', left: '-15%', width: 380, height: 380, background: 'radial-gradient(circle,rgba(124,108,255,.4),transparent 70%)', filter: 'blur(30px)', opacity: 0.5 }} />
        <div className="flex items-center gap-2.5 relative">
          <div className="w-[34px] h-[34px] rounded-[10px] flex items-center justify-center text-white" style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-2))', boxShadow: '0 3px 14px var(--accent-glow)' }}><OpenRiskLogo size={20} /></div>
          <span className="disp text-[19px] font-bold text-white">OpenRisk</span>
        </div>
        <div className="relative flex-1 flex items-center justify-center">
          <div className="relative" style={{ width: 260, height: 260 }}>
            <div className="absolute inset-0 flex items-center justify-center">
              <div className="w-[60px] h-[60px] rounded-[18px] flex items-center justify-center text-white" style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-2))', boxShadow: '0 6px 30px var(--accent-glow)' }}><OpenRiskLogo size={30} /></div>
            </div>
            {[0, 1, 2].map((i) => (
              <div key={i} className="absolute inset-0 rounded-full" style={{ border: '1px solid rgba(255,255,255,.12)', transform: `scale(${0.55 + i * 0.22})` }} />
            ))}
            {ORBIT_NODES.map(([c, x, y], i) => (
              <div key={i} className="absolute rounded-full" style={{ left: x, top: y, width: 14, height: 14, background: c, boxShadow: `0 0 12px ${c}`, animation: `or-float ${4 + i}s ease-in-out infinite` }} />
            ))}
          </div>
        </div>
        <div className="relative" style={{ minHeight: 96 }}>
          <div style={{ opacity: qShow ? 1 : 0, transform: qShow ? 'translateY(0)' : 'translateY(6px)', transition: 'opacity .35s ease, transform .35s ease' }}>
            <div className="text-[17px] font-medium text-white leading-relaxed" style={{ letterSpacing: '-.01em' }}>“{quote}”</div>
            <div className="text-[13px] mt-2.5" style={{ color: 'rgba(255,255,255,.5)' }}>— {author}</div>
          </div>
        </div>
      </div>

      {/* right */}
      <div className="flex-1 flex items-center justify-center p-8" style={{ background: 'var(--bg-app)' }}>
        <div className="w-full max-w-[380px]" style={{ animation: 'or-fadeup .4s ease' }}>
          {view === 'login' && <LoginForm onRegister={() => setView('register')} onMfa={() => setView('mfa')} />}
          {view === 'register' && <RegisterForm onLogin={() => setView('login')} onMfa={() => setView('mfa')} />}
          {view === 'mfa' && <MfaForm onBack={() => setView('login')} />}
        </div>
      </div>
    </div>
  );
}

function Label({ children }: { children: React.ReactNode }) {
  return <label className="block text-[12.5px] font-medium text-ink-soft mb-[7px]">{children}</label>;
}
const inputCls = 'w-full h-11 px-3.5 rounded-[11px] text-[14px] text-ink outline-none focus:border-[var(--accent)]';
const inputStyle: React.CSSProperties = { border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' };
const primaryBtn = 'w-full h-[46px] rounded-xl text-[14px] font-semibold text-white';
const primaryStyle: React.CSSProperties = { background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))', boxShadow: '0 4px 16px var(--accent-glow)' };

function LoginForm({ onRegister, onMfa }: { onRegister: () => void; onMfa: () => void }) {
  const L = useUIStrings();
  const navigate = useNavigate();
  const login = useAuthStore((s) => s.login);
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [show, setShow] = useState(false);
  const [loading, setLoading] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      await login(email, password);
      toast.success('Welcome back to OpenRisk');
      navigate('/');
    } catch {
      toast.error('Incorrect email or password. Please check and try again.');
    } finally {
      setLoading(false);
    }
  };

  return (
    <form onSubmit={submit}>
      <h1 className="disp text-[24px] font-bold text-ink mb-1.5">{L.welcome}</h1>
      <div className="text-[14px] text-ink-soft mb-[26px]">{L.welcomeSub}</div>
      <div className="mb-[15px]"><Label>{L.email}</Label><input type="email" value={email} onChange={(e) => setEmail(e.target.value)} autoFocus className={inputCls} style={inputStyle} /></div>
      <div className="mb-[15px]">
        <Label>{L.password}</Label>
        <div className="relative">
          <input type={show ? 'text' : 'password'} value={password} onChange={(e) => setPassword(e.target.value)} className={inputCls} style={inputStyle} />
          <button type="button" onClick={() => setShow((v) => !v)} className="absolute right-2.5 top-[11px] w-[26px] h-[22px] flex items-center justify-center text-ink-muted" aria-label="Toggle password">{show ? <EyeOff size={17} /> : <Eye size={17} />}</button>
        </div>
      </div>
      <div className="flex items-center justify-between my-1 mb-5">
        <label className="flex items-center gap-[7px] text-[12.5px] text-ink-soft cursor-pointer"><input type="checkbox" style={{ accentColor: 'var(--accent)' }} />{L.rememberMe}</label>
        <a href="#" onClick={(e) => e.preventDefault()} className="text-[12.5px] font-medium">{L.forgot}</a>
      </div>
      <button type="submit" disabled={loading} className={primaryBtn} style={{ ...primaryStyle, opacity: loading ? 0.7 : 1 }}>{loading ? '…' : L.signin}</button>
      <div className="flex items-center gap-3 my-[18px]"><div className="flex-1 h-px" style={{ background: 'var(--border)' }} /><span className="text-[12px] text-ink-muted">{L.orSep}</span><div className="flex-1 h-px" style={{ background: 'var(--border)' }} /></div>
      <div className="flex gap-2.5 mb-2">
        {['Google', 'GitHub'].map((p) => (
          <button key={p} type="button" className="flex-1 h-11 rounded-[11px] text-[13px] font-semibold text-ink flex items-center justify-center gap-2 hover:bg-hover transition-colors" style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}>{p}</button>
        ))}
      </div>
      <div className="text-center text-[13px] text-ink-soft mt-[18px]">{L.noAccount}{' '}<a href="#" onClick={(e) => { e.preventDefault(); onRegister(); }} className="font-semibold">{L.createAccount}</a></div>
      <div className="text-center mt-2"><a href="#" onClick={(e) => { e.preventDefault(); onMfa(); }} className="text-[11.5px] text-ink-muted">{L.mfaTitle} →</a></div>
    </form>
  );
}

function RegisterForm({ onLogin, onMfa }: { onLogin: () => void; onMfa: () => void }) {
  const L = useUIStrings();
  return (
    <form onSubmit={(e) => { e.preventDefault(); onMfa(); }}>
      <h1 className="disp text-[24px] font-bold text-ink mb-1.5">{L.registerTitle}</h1>
      <div className="text-[14px] text-ink-soft mb-[26px]">{L.registerSub}</div>
      <div className="flex gap-3">
        <div className="flex-1 mb-[15px]"><Label>{L.firstName}</Label><input className={inputCls} style={inputStyle} /></div>
        <div className="flex-1 mb-[15px]"><Label>{L.lastName}</Label><input className={inputCls} style={inputStyle} /></div>
      </div>
      <div className="mb-[15px]"><Label>{L.email}</Label><input type="email" className={inputCls} style={inputStyle} /></div>
      <div className="mb-[15px]"><Label>{L.password}</Label><input type="password" className={inputCls} style={inputStyle} /></div>
      <div className="flex gap-1.5 mb-[18px]">{[0, 1, 2, 3].map((i) => <div key={i} className="flex-1 h-1 rounded" style={{ background: i < 2 ? 'var(--high)' : 'var(--bg-hover)' }} />)}</div>
      <button type="submit" className={primaryBtn} style={primaryStyle}>{L.createAccount}</button>
      <div className="text-center text-[13px] text-ink-soft mt-[18px]">{L.haveAccount}{' '}<a href="#" onClick={(e) => { e.preventDefault(); onLogin(); }} className="font-semibold">{L.signinLink}</a></div>
    </form>
  );
}

function MfaForm({ onBack }: { onBack: () => void }) {
  const L = useUIStrings();
  const navigate = useNavigate();
  const [otp, setOtp] = useState<string[]>(Array(6).fill(''));
  const setDigit = (i: number, v: string, el: HTMLInputElement) => {
    const d = v.replace(/\D/g, '').slice(-1);
    setOtp((o) => { const n = [...o]; n[i] = d; return n; });
    if (d && el.nextElementSibling) (el.nextElementSibling as HTMLInputElement).focus();
  };
  return (
    <form onSubmit={(e) => { e.preventDefault(); navigate('/'); }}>
      <div className="w-[52px] h-[52px] rounded-[15px] flex items-center justify-center mb-5" style={{ background: 'var(--accent-soft)', color: 'var(--accent)' }}><Lock size={26} /></div>
      <h1 className="disp text-[23px] font-bold text-ink mb-1.5">{L.mfaTitle}</h1>
      <div className="text-[14px] text-ink-soft mb-[26px]">{L.mfaSub}</div>
      <div className="flex gap-2.5 mb-[22px]">
        {otp.map((d, i) => (
          <input key={i} value={d} maxLength={1} inputMode="numeric" onChange={(e) => setDigit(i, e.target.value, e.target)} className="mono flex-1 h-14 text-center text-[22px] font-bold rounded-xl outline-none text-ink" style={{ border: `1.5px solid ${d ? 'var(--accent)' : 'var(--border-strong)'}`, background: 'var(--bg-elevated)' }} />
        ))}
      </div>
      <button type="submit" className={primaryBtn} style={primaryStyle}>{L.verify}</button>
      <div className="flex items-center justify-between mt-[18px]">
        <a href="#" onClick={(e) => { e.preventDefault(); onBack(); }} className="text-[12.5px] font-medium">← {L.signinLink}</a>
        <a href="#" onClick={(e) => e.preventDefault()} className="text-[12.5px] font-medium">{L.resend}</a>
      </div>
    </form>
  );
}
