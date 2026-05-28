import { BadgePlus, CheckCircle2, Loader2, LockKeyhole, Mail, MessageCircle, ShieldCheck, UserRound, UsersRound } from "lucide-react";
import { FormEvent, useState } from "react";
import { Field } from "../ui";
import type { AuthMode } from "../types";
export function AuthView({
  busy,
  onLogin,
  onRegister
}: {
  busy: boolean;
  onLogin: (input: { email: string; password: string }) => Promise<void>;
  onRegister: (input: { aim_id: string; email: string; nickname: string; password: string }) => Promise<void>;
}) {
  const [mode, setMode] = useState<AuthMode>("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [aimID, setAimID] = useState("");
  const [nickname, setNickname] = useState("");

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (mode === "login") {
      await onLogin({ email, password });
      return;
    }
    await onRegister({ aim_id: aimID, email, nickname, password });
    setMode("login");
  };

  return (
    <main className="auth-screen">
      <section className="auth-copy" aria-label="AIM">
        <div className="brand-row">
          <div className="brand-mark">A</div>
          <div>
            <h1>AIM</h1>
          </div>
        </div>
        <div className="auth-signal">
          <div className="signal-line">
            <ShieldCheck size={18} />
            <span>AI助手</span>
          </div>
          <div className="signal-line">
            <UsersRound size={18} />
            <span>群聊</span>
          </div>
          <div className="signal-line">
            <MessageCircle size={18} />
            <span>历史记录</span>
          </div>
        </div>
      </section>

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
                <input required value={aimID} onChange={(event) => setAimID(event.target.value)} placeholder="xqe_0422" />
              </Field>
              <Field icon={<UserRound size={18}></UserRound>} label="昵称">
                <input required value={nickname} onChange={(event) => setNickname(event.target.value)} placeholder="小青" />
              </Field>
            </>
          )}
          <Field icon={<Mail size={18}></Mail>} label="邮箱">
            <input required type="email" value={email} onChange={(event) => setEmail(event.target.value)} placeholder="请输入邮箱" />
          </Field>
          <Field icon={<LockKeyhole size={18}></LockKeyhole>} label="密码">
            <input required type="password" value={password} onChange={(event) => setPassword(event.target.value)} placeholder="请输入密码" />
          </Field>
          <button className="primary-action" disabled={busy} type="submit">
            {busy ? <Loader2 className="spin" size={18} /> : <CheckCircle2 size={18} />}
            {mode === "login" ? "登录 AIM" : "创建账号"}
          </button>
        </form>
      </section>
    </main>
  );
}


