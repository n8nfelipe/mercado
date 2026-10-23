import { LoginForm } from "./login-form";

export default function LoginPage() {
  return (
    <div className="auth-page">
      <div className="auth-card">
        <p className="eyebrow">Conta</p>
        <h1>Entrar</h1>
        <LoginForm />
      </div>
    </div>
  );
}
