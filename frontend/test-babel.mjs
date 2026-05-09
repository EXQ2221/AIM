import * as babylon from '@babel/parser';

// Test 1: The exact Mail self-closing pattern
const tests = [
    {
        name: 'Self-closing in prop (Mail)',
        code: `
function Field({icon, label, children}) { return <div>{children}</div>; }
function Mail(props) { return null; }
const x = <Field icon={<Mail size={18} />} label="邮箱"><input /></Field>;
`
    },
    {
        name: 'Non-self-closing (Mail)',  
        code: `
function Field({icon, label, children}) { return <div>{children}</div>; }
function Mail(props) { return null; }
const x = <Field icon={<Mail size={18}></Mail>} label="邮箱"><input /></Field>;
`
    },
    {
        name: 'Full auth form fragment',
        code: `
import React from 'react';
import { Loader2, CheckCircle2 } from 'lucide-react';
function Field({ icon, label, children }) { return <div><label>{label}</label>{children}</div>; }
function BadgePlus(p) { return null; }
function UserRound(p) { return null; }
function Mail(p) { return null; }
function LockKeyhole(p) { return null; }

export default function AuthView({ mode, setMode, submit, busy }) {
  const [aimID, setAimID] = React.useState('');
  const [nickname, setNickname] = React.useState('');
  const [email, setEmail] = React.useState('');
  const [password, setPassword] = React.useState('');

  return (
    <section className="auth-card">
      <div className="segmented">
        <button className={mode === "login" ? "active" : ""} type="button" onClick={() => setMode("login")}>
          登录
        </button>
        <button className={mode === "register" ? "active" : ""} type="button" onClick={() => setMode("register")}>
          注册
        </button>
      </div>
      <form className="stack-form" onSubmit={submit}>
        {mode === "register" && (
          <>
            <Field icon={<BadgePlus size={18}></BadgePlus>} label="AIM ID">
              <input required value={aimID} onChange={(e) => setAimID(e.target.value)} placeholder="xqe_0422" />
            </Field>
            <Field icon={<UserRound size={18}></UserRound>} label="昵称">
              <input required value={nickname} onChange={(e) => setNickname(e.target.value)} placeholder="小青" />
            </Field>
          </>
        )}
        <Field icon={<Mail size={18} />} label="邮箱">
          <input required type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="demo@example.com" />
        </Field>
        <Field icon={<LockKeyhole size={18}></LockKeyhole>} label="密码">
          <input required type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="Password123!" />
        </Field>
        <button className="primary-action" disabled={busy} type="submit">
          {busy ? <Loader2 className="spin" size={18} /> : <CheckCircle2 size={18} />}
          {mode === "login" ? "登录 AIM" : "创建账号"}
        </button>
      </form>
    </section>
  );
}
`
    },
];

for (const t of tests) {
    try {
        babylon.parse(t.code, { sourceType: 'module', plugins: ['jsx', 'typescript', 'decorators-legacy'] });
        console.log(`"${t.name}": PASS ✓`);
    } catch(e) {
        console.log(`"${t.name}": FAIL ✗ - ${e.message.split('\n')[0]}`);
        if (e.loc) console.log(`  Location: line ${e.loc.line}, col ${e.loc.column}`);
    }
}
