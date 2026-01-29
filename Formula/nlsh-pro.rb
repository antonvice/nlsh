class NlshPro < Formula
  desc "Natural Language Shell Pro"
  homepage "https://github.com/antonvice/nlsh-pro"
  url "https://github.com/antonvice/nlsh-pro/archive/v1.0.0.tar.gz" # Placeholder, update on release
  sha256 "0000000000000000000000000000000000000000000000000000000000000000" # Placeholder
  license "MIT"

  depends_on "go" => :build
  depends_on "fish" => :optional

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w")
    
    # Install fish functions (Homebrew specific pathing usually requires user setup)
    # We can install them to share/fish/vendor_functions.d if we want robust auto-loading
    (share/"fish/vendor_functions.d").install "functions/fish_command_not_found.fish"
  end

  def caveats
    <<~EOS
      NLSH-Pro installed!
      
      To enable the interceptor, you may need to source your fish config:
        source ~/.config/fish/config.fish
        
      If you installed fish via Homebrew, the functions should autoload.
      Otherwise, copy the function manually:
        cp #{share}/fish/vendor_functions.d/fish_command_not_found.fish ~/.config/fish/functions/
    EOS
  end

  test do
    assert_match "NLSH", shell_output("#{bin}/nlsh-pro status")
  end
end
