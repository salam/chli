class Chli < Formula
  desc "Unified CLI for Swiss government open data"
  homepage "https://github.com/matthiasak/chli"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/matthiasak/chli/releases/download/v#{version}/chli_#{version}_darwin_arm64.tar.gz"
    else
      url "https://github.com/matthiasak/chli/releases/download/v#{version}/chli_#{version}_darwin_amd64.tar.gz"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/matthiasak/chli/releases/download/v#{version}/chli_#{version}_linux_arm64.tar.gz"
    else
      url "https://github.com/matthiasak/chli/releases/download/v#{version}/chli_#{version}_linux_amd64.tar.gz"
    end
  end

  def install
    bin.install "chli"
    bash_completion.install "completions/chli.bash" => "chli"
    zsh_completion.install "completions/chli.zsh" => "_chli"
    fish_completion.install "completions/chli.fish"
  end

  test do
    system "#{bin}/chli", "version"
  end
end
