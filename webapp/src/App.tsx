import {
  useFliptBoolean,
  useFliptSelector,
} from "@flipt-io/flipt-client-react";

const themes = {
  beach: {
    image:
      "url('https://images.unsplash.com/photo-1507525428034-b723cf961d3e?q=80&w=1544')",
    color: "bg-blue-400",
  },
  city: {
    image:
      "url('https://images.unsplash.com/photo-1477959858617-67f85cf4f1df?q=80&w=1544')",
    color: "bg-gray-800",
  },
  mountain: {
    image:
      "url('https://images.unsplash.com/photo-1501785888041-af3ef285b470?q=80&w=1544')",
    color: "bg-green-700",
  },
  snowboard: {
    image:
      "url('https://images.unsplash.com/photo-1478700485868-972b69dc3fc4?q=80&w=1544')",
    color: "bg-white",
  },
  default: {
    image:
      "url('https://images.unsplash.com/photo-1758132123976-6730692335f7?q=80&w=1544')",
    color: "bg-white",
  },
  none: {
    image: "",
    color: "bg-gray-300",
  },
};

function App() {
  const sale = useFliptBoolean("sale", false, "user-123", {});
  const themeKey = useFliptSelector((client, isLoading, error) => {
    if (isLoading) {
      return "none";
    }
    if (client && !isLoading && !error) {
      try {
        return client.evaluateVariant({
          flagKey: "theme",
          entityId: "user-123",
          context: {
            month: (new Date().getMonth() + 1).toFixed(0),
          },
        }).variantKey;
      } catch (e) {
        console.error("Error evaluating variant flag theme:", e);
      }
    }
    return "default";
  });

  // @ts-ignore
  const theme = themes[themeKey] || themes.default;
  return (
    <div
      className={`h-full bg-cover bg-center ${theme.color}`}
      style={{ backgroundImage: theme.image }}
    >
      {sale && (
        <div className="bg-yellow-300 text-black p-4 text-center font-bold">
          Season Sale! Book your dream vacation now!
        </div>
      )}
      <header className="flex justify-between items-center p-6 bg-white shadow text-gray-600">
        <div className="text-2xl font-bold "> TravelCo </div>
        <nav>
          <a href="#" className="px-3 text-gray-600">
            Contact
          </a>
        </nav>
      </header>
      <section className="m-auto h-3/5 w-2/5 flex flex-col justify-end items-center text-white text-center ">
        <h1 className="text-4xl font-bold mb-4">Your Next Adventure Awaits</h1>
        <button className="bg-white text-black px-6 py-3 font-semibold rounded shadow-xl">
          Explore Now
        </button>
      </section>
    </div>
  );
}

export default App;
