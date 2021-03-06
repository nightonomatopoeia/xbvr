package scrape

import (
	"strconv"
	"strings"
	"sync"

	"github.com/gocolly/colly"
	"github.com/mozillazg/go-slugify"
	"github.com/robertkrimen/otto"
	"github.com/thoas/go-funk"
	"github.com/tidwall/gjson"
	"github.com/xbapps/xbvr/pkg/models"
	"gopkg.in/resty.v1"
)

func VirtualRealPornSite(wg *sync.WaitGroup, updateSite bool, knownScenes []string, out chan<- models.ScrapedScene, scraperID string, siteID string, URL string) error {
	defer wg.Done()
	logScrapeStart(scraperID, siteID)

	sceneCollector := createCollector("virtualrealporn.com", "virtualrealtrans.com")
	siteCollector := createCollector("virtualrealporn.com", "virtualrealtrans.com")
	castCollector := createCollector("virtualrealporn.com", "virtualrealtrans.com")
	castCollector.AllowURLRevisit = true

	sceneCollector.OnHTML(`html`, func(e *colly.HTMLElement) {
		sc := models.ScrapedScene{}
		sc.SceneType = "VR"
		sc.Studio = "VirtualRealPorn"
		sc.Site = siteID
		sc.HomepageURL = strings.Split(e.Request.URL.String(), "?")[0]

		var tmpCast []string

		// Scene ID - get from URL
		e.ForEach(`link[rel=shortlink]`, func(id int, e *colly.HTMLElement) {
			sc.SiteID = strings.Split(e.Attr("href"), "?p=")[1]
			sc.SceneID = slugify.Slugify(sc.Site) + "-" + sc.SiteID
		})

		// Title
		e.ForEach(`title`, func(id int, e *colly.HTMLElement) {
			sc.Title = e.Text
			sc.Title = strings.TrimSpace(strings.Replace(sc.Title, "▷ ", "", -1))
			sc.Title = strings.TrimSpace(strings.Replace(sc.Title, " - VirtualRealPorn.com", "", -1))
			sc.Title = strings.TrimSpace(strings.Replace(sc.Title, " - VirtualRealTrans.com", "", -1))
		})

		// Cover URLs
		e.ForEach(`meta[property="og:image"]`, func(id int, e *colly.HTMLElement) {
			if id == 0 {
				sc.Covers = append(sc.Covers, strings.Split(e.Request.AbsoluteURL(e.Attr("content")), "?")[0])
			}
		})

		// Gallery
		e.ForEach(`a.w-gallery-tnail`, func(id int, e *colly.HTMLElement) {
			sc.Gallery = append(sc.Gallery, e.Request.AbsoluteURL(strings.Split(e.Attr("href"), "?")[0]))
		})

		// Tags
		e.ForEach(`a.g-btn span`, func(id int, e *colly.HTMLElement) {
			sc.Tags = append(sc.Tags, strings.TrimSpace(e.Text))
		})

		// Duration / Release date / Synopsis
		e.ForEach(`script[type='application/ld+json'][class!='yoast-schema-graph']`, func(id int, e *colly.HTMLElement) {
			vm := otto.New()

			script := "sin = " + e.Text
			script = script + ";\nduration = sin['duration']; datePublished = sin['datePublished']; desc = sin['description'];"
			script = script + "cast = []; sin['actors'].map(function(o){cast.push(o.url)});"

			vm.Run(script)

			out1, _ := vm.Get("duration")
			duration, _ := out1.ToString()
			sc.Duration, _ = strconv.Atoi(strings.Split(duration, ":")[0])

			out2, _ := vm.Get("datePublished")
			relDate, _ := out2.ToString()
			sc.Released = relDate

			out3, _ := vm.Get("desc")
			desc, _ := out3.ToString()
			sc.Synopsis = desc

			out4, _ := vm.Get("cast")
			cast, _ := out4.Export()
			castx, ok := cast.([]string)

			if ok {
				for i := range castx {
					tmpCast = append(tmpCast, castx[i])
				}
			}
		})

		e.ForEach(`dl8-video source`, func(id int, e *colly.HTMLElement) {
			if id == 0 {
				origURL := e.Attr("src")
				fragmentName := strings.Split(origURL, "/")

				fpName := strings.Split(fragmentName[len(fragmentName)-1], "?")[0]
				prefix := siteID + ".com_-_"
				if strings.HasPrefix(fpName, prefix) {
					fpName = strings.Split(fpName, prefix)[1]
					fpName = strings.SplitN(fpName, "_-_Trailer", 2)[0]
					siteIDAcronym := "VRP"
					if siteID == "VirtualRealTrans" {
						siteIDAcronym = "VRT"
					}

					var outFilenames []string

					// Playstation VR
					outFilenames = append(outFilenames, siteIDAcronym+"_"+fpName+"_Trailer_PS4_180_sbs.mp4") // Trailer
					outFilenames = append(outFilenames, siteID+"_"+fpName+"_3K_180_sbs.mp4")                 // PS4
					outFilenames = append(outFilenames, siteIDAcronym+"_"+fpName+"_180_sbs.mp4")             // PS4 (older videos)
					outFilenames = append(outFilenames, siteID+".com_-_"+fpName+"_-_180_sbs.mp4")            // PS4 (oldest videos)
					outFilenames = append(outFilenames, siteID+"_"+fpName+"_4096x2040_180_sbs.mp4")          // PS4 Pro
					outFilenames = append(outFilenames, siteIDAcronym+"_"+fpName+"_Pro_180_sbs.mp4")         // PS4 Pro (older videos)
					outFilenames = append(outFilenames, siteID+".com_-_"+fpName+"_-_Pro_180_sbs.mp4")        // PS4 Pro (oldest videos)

					// Oculus Go / Quest
					outFilenames = append(outFilenames, siteID+".com_-_"+fpName+"_-_Trailer.mp4")       // Trailer (same for Oculus Rift (S) / Vive / Windows MR)
					outFilenames = append(outFilenames, siteID+"_"+fpName+"_4864_180x180_3dh.mp4")      // 4K+
					outFilenames = append(outFilenames, siteID+"_-_"+fpName+"_-_h264P_180x180_3dh.mp4") // 4K+ (older videos)
					outFilenames = append(outFilenames, siteID+"_"+fpName+"_4K_30M_180x180_3dh.mp4")    // 4K HQ (same for Gear VR / Daydream and Oculus Rift (S) / Vive / Windows MR)
					outFilenames = append(outFilenames, siteID+"_"+fpName+"_4K_h265_180x180_3dh.mp4")   // 4K h265 (same for Oculus Rift (S) / Vive / Windows MR)
					outFilenames = append(outFilenames, siteID+"_-_"+fpName+"_-_vp9_180x180_3dh.mp4")   // 4K VP9 (older videos; same for Gear VR / Daydream and Oculus Rift (S) / Vive / Windows MR)
					outFilenames = append(outFilenames, siteID+"_-_"+fpName+"_-_180x180_3dh.mp4")       // 4K h264 (older videos; same for Gear VR / Daydream)

					// Gear VR / Daydream
					outFilenames = append(outFilenames, siteID+"_-_"+fpName+"_-_Trailer_Streaming_3dh.mp4") // Trailer
					outFilenames = append(outFilenames, siteID+"_"+fpName+"_4K_180x180_3dh.mp4")            // 4K (same for Smartphone)

					// Smartphone
					outFilenames = append(outFilenames, siteID+".com_-_"+fpName+"_-_Trailer_-_Smartphone.mp4") // Trailer
					outFilenames = append(outFilenames, siteID+"_"+fpName+"_1920_180x180_3dh.mp4")             // Full HD (same for Oculus Rift (S) / Vive / Windows MR)
					outFilenames = append(outFilenames, siteID+".com_-_"+fpName+"_-_1920.mp4")                 // Full HD (older videos; same for Oculus Rift (S) / Vive / Windows MR)

					// Oculus Rift (S) / Vive / Windows MR
					outFilenames = append(outFilenames, siteID+"_"+fpName+"_5K_30M_180x180_3dh.mp4")     // 5K HQ
					outFilenames = append(outFilenames, siteID+"_"+fpName+"_5K_180x180_3dh.mp4")         // 5K
					outFilenames = append(outFilenames, siteID+"_-_"+fpName+"_-_5K_180x180_3dh.mp4")     // 5K (older videos)
					outFilenames = append(outFilenames, siteID+".com_-_"+fpName+"_-_5K_180x180_3dh.mp4") // 5K (before site update)
					outFilenames = append(outFilenames, siteID+".com_-_"+fpName+"_-_h264.mp4")           // 4K 264 (older videos)

					sc.Filenames = outFilenames
				}
			}
		})

		ctx := colly.NewContext()
		ctx.Put("scene", &sc)

		for i := range tmpCast {
			castCollector.Request("GET", tmpCast[i], nil, ctx, nil)
		}

		out <- sc
	})

	castCollector.OnHTML(`html`, func(e *colly.HTMLElement) {
		sc := e.Request.Ctx.GetAny("scene").(*models.ScrapedScene)

		var name string
		e.ForEach(`h1.model-title`, func(id int, e *colly.HTMLElement) {
			name = strings.Split(e.Text, " (")[0]
		})

		var gender string
		e.ForEach(`div.model-info div.one-half div`, func(id int, e *colly.HTMLElement) {
			if strings.Split(e.Text, " ")[0] == "Gender" {
				gender = strings.Split(e.Text, " ")[1]
			}
		})

		if gender == "Female" || gender == "Transgender" {
			sc.Cast = append(sc.Cast, name)
		}
	})

	siteCollector.OnHTML(`a.w-portfolio-item-anchor`, func(e *colly.HTMLElement) {
		sceneURL := e.Request.AbsoluteURL(e.Attr("href"))

		// If scene exist in database, there's no need to scrape
		if !funk.ContainsString(knownScenes, sceneURL) {
			sceneCollector.Visit(sceneURL)
		}
	})

	// Request scenes via ajax interface
	r, err := resty.R().
		SetHeader("User-Agent", userAgent).
		SetHeader("Accept", "application/json, text/javascript, */*; q=0.01").
		SetHeader("Referer", URL).
		SetHeader("X-Requested-With", "XMLHttpRequest").
		SetHeader("Authority", scraperID+".com").
		SetFormData(map[string]string{
			"action": "get_videos_list",
			"p":      "1",
			"vpp":    "1000",
			"sq":     "",
			"so":     "date-DESC",
			"pid":    "8",
		}).
		Post("https://" + scraperID + ".com/wp-admin/admin-ajax.php")

	if err == nil || r.StatusCode() == 200 {
		urls := gjson.Get(r.String(), "data.movies.#.permalink").Array()
		for i := range urls {
			sceneURL := urls[i].String()
			if !funk.ContainsString(knownScenes, sceneURL) {
				sceneCollector.Visit(sceneURL)
			}
		}
	}

	siteCollector.Visit(URL)

	if updateSite {
		updateSiteLastUpdate(scraperID)
	}
	logScrapeFinished(scraperID, siteID)
	return nil
}

func VirtualRealPorn(wg *sync.WaitGroup, updateSite bool, knownScenes []string, out chan<- models.ScrapedScene) error {
	return VirtualRealPornSite(wg, updateSite, knownScenes, out, "virtualrealporn", "VirtualRealPorn", "https://virtualrealporn.com/")
}
func VirtualRealTrans(wg *sync.WaitGroup, updateSite bool, knownScenes []string, out chan<- models.ScrapedScene) error {
	return VirtualRealPornSite(wg, updateSite, knownScenes, out, "virtualrealtrans", "VirtualRealTrans", "https://virtualrealtrans.com/")
}

func init() {
	registerScraper("virtualrealporn", "VirtualRealPorn", "https://twivatar.glitch.me/virtualrealporn", VirtualRealPorn)
	registerScraper("virtualrealtrans", "VirtualRealTrans", "https://twivatar.glitch.me/virtualrealporn", VirtualRealTrans)
}
