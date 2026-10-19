// Everything that differs between the MelonLoader and BepInEx builds lives here and in Entry.cs.
// Build with -p:Loader=BepInEx for the BepInEx / Gale version (defines BEPINEX).

// The two loaders generate the game's interop assemblies with different namespaces:
// MelonLoader prefixes them (Il2Cpp.PlayerCharacter, Il2CppRewired.Player), BepInEx doesn't.
#if BEPINEX
global using RW = Rewired;
#else
global using Il2Cpp;
global using RW = Il2CppRewired;
#endif

namespace BigWalkTeleport
{
    static class ModLog
    {
#if BEPINEX
        internal static BepInEx.Logging.ManualLogSource Source;
        public static void Msg(string message) => Source?.LogInfo(message);
        public static void Warning(string message) => Source?.LogWarning(message);
        public static void Error(string message) => Source?.LogError(message);
#else
        public static void Msg(string message) => MelonLoader.MelonLogger.Msg(message);
        public static void Warning(string message) => MelonLoader.MelonLogger.Warning(message);
        public static void Error(string message) => MelonLoader.MelonLogger.Error(message);
#endif
    }

    // A setting stored by the loader: MelonPreferences.cfg for MelonLoader,
    // BepInEx/config/tri21692.BigWalkTeleport.cfg for BepInEx.
    class Setting<T>
    {
#if BEPINEX
        readonly BepInEx.Configuration.ConfigEntry<T> entry;
        public Setting(BepInEx.Configuration.ConfigEntry<T> entry) => this.entry = entry;
#else
        readonly MelonLoader.MelonPreferences_Entry<T> entry;
        public Setting(MelonLoader.MelonPreferences_Entry<T> entry) => this.entry = entry;
#endif
        public T Value { get => entry.Value; set => entry.Value = value; }
    }

    class SettingsCategory
    {
#if BEPINEX
        readonly BepInEx.Configuration.ConfigFile file;
        const string Section = "BigWalkTeleport";
        public SettingsCategory(BepInEx.Configuration.ConfigFile file) { this.file = file; Settings.File = file; }
        public Setting<T> CreateEntry<T>(string id, T defaultValue, string description) =>
            new(file.Bind(Section, id, defaultValue, description));
#else
        readonly MelonLoader.MelonPreferences_Category category;
        public SettingsCategory(MelonLoader.MelonPreferences_Category category) => this.category = category;
        public Setting<T> CreateEntry<T>(string id, T defaultValue, string description) =>
            new(category.CreateEntry(id, defaultValue, description));
#endif
    }

    static class Settings
    {
#if BEPINEX
        internal static BepInEx.Configuration.ConfigFile File;
        public static void Save() => File?.Save();
#else
        public static void Save() => MelonLoader.MelonPreferences.Save();
#endif
    }
}
