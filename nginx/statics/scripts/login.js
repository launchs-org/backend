const auth = new AuthBase('/auth/');

async function Init() {
    // ログインしているか確認
    if (await auth.GetInfo() != null) {
        // ログイン済み
        window.location.href = "./home.html";
    }
}

function OauthLogin(provider) {
    auth.OauthLogin(provider,LoginSuccess);
}

function LoginSuccess() {
    window.location.href = "./home.html";
}

Init();