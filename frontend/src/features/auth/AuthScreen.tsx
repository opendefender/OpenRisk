// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Auth (OpenRisk.dc.html §6.1): split-screen 45/55 — animated orbit + Schneier
// quote on the left, Login / Register / MFA forms on the right. The Login form is
// wired to the real auth store; Register/MFA are the design flow.

import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import axios from 'axios';
import { Eye, EyeOff, Sun, Moon } from 'lucide-react';
import { api } from '../../lib/api';
import { useAuthStore } from '../../hooks/useAuthStore';
import { landingForBusinessRole } from '../../shared/navModel';
import { useUIStore } from '../../store/uiStore';
import { useUIStrings } from '../../shared/uiStrings';
import { OpenRiskLogo } from '../../shared/Logo';

type View = 'login' | 'register';

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
          {view === 'login' && <LoginForm onRegister={() => setView('register')} />}
          {view === 'register' && <RegisterForm onLogin={() => setView('login')} />}
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

function LoginForm({ onRegister }: { onRegister: () => void }) {
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
      // Land each GRC business role on a screen relevant to its job (mirrors the
      // backend default landing); admins/root land on the main dashboard.
      const businessRole = useAuthStore.getState().user?.business_role;
      navigate(landingForBusinessRole(businessRole));
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
      <div className="mb-[15px]"><Label>{L.email}</Label><input data-testid="login-email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} autoFocus className={inputCls} style={inputStyle} /></div>
      <div className="mb-[15px]">
        <Label>{L.password}</Label>
        <div className="relative">
          <input data-testid="login-password" type={show ? 'text' : 'password'} value={password} onChange={(e) => setPassword(e.target.value)} className={inputCls} style={inputStyle} />
          <button type="button" onClick={() => setShow((v) => !v)} className="absolute right-2.5 top-[11px] w-[26px] h-[22px] flex items-center justify-center text-ink-muted" aria-label="Toggle password">{show ? <EyeOff size={17} /> : <Eye size={17} />}</button>
        </div>
      </div>
      <div className="flex items-center justify-between my-1 mb-5">
        <label className="flex items-center gap-[7px] text-[12.5px] text-ink-soft cursor-pointer"><input type="checkbox" style={{ accentColor: 'var(--accent)' }} />{L.rememberMe}</label>
        <a href="#" onClick={(e) => e.preventDefault()} className="text-[12.5px] font-medium">{L.forgot}</a>
      </div>
      <button data-testid="login-submit" type="submit" disabled={loading} className={primaryBtn} style={{ ...primaryStyle, opacity: loading ? 0.7 : 1 }}>{loading ? '…' : L.signin}</button>
      <div className="flex items-center gap-3 my-[18px]"><div className="flex-1 h-px" style={{ background: 'var(--border)' }} /><span className="text-[12px] text-ink-muted">{L.orSep}</span><div className="flex-1 h-px" style={{ background: 'var(--border)' }} /></div>
      <div className="flex gap-2.5 mb-2">
        {['Google', 'GitHub'].map((p) => (
          <button key={p} type="button" className="flex-1 h-11 rounded-[11px] text-[13px] font-semibold text-ink flex items-center justify-center gap-2 hover:bg-hover transition-colors" style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}>{p}</button>
        ))}
      </div>
      <div className="text-center text-[13px] text-ink-soft mt-[18px]">{L.noAccount}{' '}<a href="#" onClick={(e) => { e.preventDefault(); onRegister(); }} className="font-semibold">{L.createAccount}</a></div>
    </form>
  );
}

// Real registration (UX-01/UX-02/UX-13): the strict minimum — full name, email,
// password (3 fields). Username + a personal workspace name are derived so the
// user isn't asked to qualify before their account exists. On success we create
// the account (POST /auth/register → org + membership), then log in and land.
function RegisterForm({ onLogin }: { onLogin: () => void }) {
  const L = useUIStrings();
  const navigate = useNavigate();
  const lang = useUIStore((s) => s.lang);
  const login = useAuthStore((s) => s.login);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const [fullName, setFullName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [show, setShow] = useState(false);
  const [loading, setLoading] = useState(false);

  const strength = Math.min(4, Math.floor(password.length / 3) + (/[^a-zA-Z0-9]/.test(password) ? 1 : 0));

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!fullName.trim()) return toast.error(tr('Indiquez votre nom.', 'Please enter your name.'));
    if (password.length < 8) return toast.error(tr('Le mot de passe doit faire au moins 8 caractères.', 'Password must be at least 8 characters.'));
    setLoading(true);
    try {
      const local = (email.split('@')[0] || 'user').replace(/[^a-zA-Z0-9_.-]/g, '');
      const username = local.length >= 3 ? local : `${local || 'user'}${Date.now().toString().slice(-4)}`;
      const company = fullName.trim() ? `${fullName.trim()}${tr(' — espace', ' — workspace')}` : `${username} workspace`;
      await api.post('/auth/register', {
        email: email.trim(),
        username,
        password,
        full_name: fullName.trim(),
        company_name: company,
      });
      // Account exists → sign in and land. Qualification comes later (UX-13).
      await login(email.trim(), password);
      toast.success(tr('Compte créé — bienvenue sur OpenRisk !', 'Account created — welcome to OpenRisk!'));
      const businessRole = useAuthStore.getState().user?.business_role;
      navigate(landingForBusinessRole(businessRole));
    } catch (err) {
      const status = axios.isAxiosError(err) ? err.response?.status : undefined;
      if (status === 409) {
        toast.error(tr('Un compte existe déjà avec cet email. Connectez-vous.', 'An account already exists for this email. Please sign in.'));
      } else if (status === 400) {
        const msg = axios.isAxiosError(err) ? (err.response?.data as { error?: string })?.error : undefined;
        toast.error(msg || tr('Vérifiez vos informations et réessayez.', 'Please check your details and try again.'));
      } else {
        toast.error(tr("La création du compte a échoué. Réessayez dans un instant.", "We couldn't create your account. Please try again shortly."));
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <form onSubmit={submit}>
      <h1 className="disp text-[24px] font-bold text-ink mb-1.5">{L.registerTitle}</h1>
      <div className="text-[14px] text-ink-soft mb-[26px]">{L.registerSub}</div>
      <div className="mb-[15px]"><Label>{tr('Nom complet', 'Full name')}</Label><input value={fullName} onChange={(e) => setFullName(e.target.value)} autoFocus className={inputCls} style={inputStyle} /></div>
      <div className="mb-[15px]"><Label>{L.email}</Label><input type="email" value={email} onChange={(e) => setEmail(e.target.value)} className={inputCls} style={inputStyle} /></div>
      <div className="mb-[15px]">
        <Label>{L.password}</Label>
        <div className="relative">
          <input type={show ? 'text' : 'password'} value={password} onChange={(e) => setPassword(e.target.value)} className={inputCls} style={inputStyle} />
          <button type="button" onClick={() => setShow((v) => !v)} className="absolute right-2.5 top-[11px] w-[26px] h-[22px] flex items-center justify-center text-ink-muted" aria-label={tr('Afficher le mot de passe', 'Toggle password')}>{show ? <EyeOff size={17} /> : <Eye size={17} />}</button>
        </div>
      </div>
      <div className="flex gap-1.5 mb-[18px]">{[0, 1, 2, 3].map((i) => <div key={i} className="flex-1 h-1 rounded" style={{ background: i < strength ? (strength >= 3 ? 'var(--low)' : 'var(--high)') : 'var(--bg-hover)' }} />)}</div>
      <button type="submit" disabled={loading} className={primaryBtn} style={{ ...primaryStyle, opacity: loading ? 0.7 : 1 }}>{loading ? '…' : L.createAccount}</button>
      <div className="text-center text-[13px] text-ink-soft mt-[18px]">{L.haveAccount}{' '}<a href="#" onClick={(e) => { e.preventDefault(); onLogin(); }} className="font-semibold">{L.signinLink}</a></div>
    </form>
  );
}
