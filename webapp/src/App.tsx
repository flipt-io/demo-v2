import {
  useFliptBoolean,
  useFliptSelector,
} from "@flipt-io/flipt-client-react";

function App() {
  const entityId = "user-123";
  const sale = useFliptBoolean("sale", false, entityId, {});
  const fallbackImage =
    "https://images.unsplash.com/photo-1758132123976-6730692335f7?q=80&w=1544";
  const themeImage = useFliptSelector((client, isLoading, error) => {
    if (isLoading) {
      return "";
    }
    if (client && !isLoading && !error) {
      try {
        return (
          JSON.parse(
            client.evaluateVariant({
              flagKey: "theme",
              entityId,
              context: {
                month: (new Date().getMonth() + 1).toFixed(0),
              },
            }).variantAttachment,
          )[0] || fallbackImage
        );
      } catch (e) {
        console.error("Error evaluating variant flag theme:", e);
      }
      return fallbackImage;
    }
  });

  return (
    <div
      className="h-full bg-cover bg-center bg-gray-300"
      style={{ backgroundImage: "url(" + themeImage + ")" }}
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
