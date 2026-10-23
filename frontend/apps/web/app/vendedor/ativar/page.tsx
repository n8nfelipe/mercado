import { OnboardingForm } from "./onboarding-form";

export default function ActivateSellerPage() {
  return (
    <div className="auth-page">
      <div className="auth-card">
        <p className="eyebrow">Vendas</p>
        <h1>Ativar conta de vendedor</h1>
        <p className="lede">Informe os dados da sua loja para publicar anúncios.</p>
        <OnboardingForm />
      </div>
    </div>
  );
}
