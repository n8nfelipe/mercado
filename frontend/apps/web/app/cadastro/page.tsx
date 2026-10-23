import { RegisterForm } from "./register-form";

export default function RegisterPage() {
  return (
    <div className="auth-page">
      <div className="auth-card">
        <p className="eyebrow">Conta</p>
        <h1>Criar conta</h1>
        <RegisterForm />
      </div>
    </div>
  );
}
